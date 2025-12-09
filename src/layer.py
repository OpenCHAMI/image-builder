from datetime import datetime
import pathmod
import sys
import os
# written modules
from image_config import ImageConfig
from utils import cmd, run_playbook
from publish import publish
import installer
import logging
from oscap import Oscap


class Layer:
    def __init__(self, args, image_config):
        self.args = args
        self.image_config = image_config
        self.logger = logging.getLogger(__name__)

    def _build_base(self, repos, modules, packages, package_groups, remove_packages, commands, copyfiles, oscap_options):
        # Set local variables
        dt_string = datetime.now().strftime("%Y%m%d%H%M%S")
        parent = self.args['parent']
        container = self.args['name']
        registry_opts_pull = self.args['registry_opts_pull']
        package_manager = self.args['pkg_man']
        if 'gpgcheck' in self.args:
            gpgcheck = self.args['gpgcheck']
        else:
            gpgcheck = True
        if 'proxy' in self.args:
            proxy = self.args['proxy']
        else:
            proxy = ""

        # container and mount name
        def buildah_handler(line):
            out.append(line)

        # Create a new container from parent
        out = []
        cmd(["buildah", "from"] + registry_opts_pull + ["--name", container + dt_string, parent], stdout_handler = buildah_handler)
        cname = out[0]

        # Copy Files
        try:
            inst.install_copyfiles(copyfiles)
        except Exception as e:
            self.logger.error(f"Error running commands: {e}")
            cmd(["buildah","rm"] + [cname])
            sys.exit("Exiting now")
        except KeyboardInterrupt:
            self.logger.error(f"Keyboard Interrupt")
            cmd(["buildah","rm"] + [cname])
            sys.exit("Exiting now ...")

        # Only mount when doing a scratch install
        if parent == "scratch":
            out = []
            cmd(["buildah", "mount"] + [cname], stdout_handler = buildah_handler)
            mname = out[0]
            self.logger.info(f"Container: {cname} mounted at {mname}")
        else:
            mname = ""

        if package_manager == "zypper":
            repo_dest = "/etc/zypp/repos.d"
        elif package_manager == "dnf":
            repo_dest = os.path.expanduser("~/.pkg_repos/yum.repos.d")
            # Create repo dest, if needed
            os.makedirs(os.path.join(mname, pathmod.sep_strip(repo_dest)), exist_ok=True)

            # Create dnf.conf file, if needed
            os.makedirs(os.path.join(mname, "etc/dnf"), exist_ok=True)
            if not os.path.exists(os.path.join(mname, "etc/dnf/dnf.conf")):
                os.mknod(os.path.join(mname, "etc/dnf/dnf.conf"), mode=0o644)

            # Add repo directory path to dnf.conf, if needed
            # Collect the contents of the file
            dnf_conf = open(os.path.join(mname, "etc/dnf/dnf.conf"), "r")
            dnf_conf_contents = dnf_conf.readlines()
            dnf_conf.close()

            ## If repodir line does not exists, add it
            if not str("reposdir=" + repo_dest + "\n") in dnf_conf_contents:
                ## If there is "[main]" section, add just the repodir
                dnf_conf = open(os.path.join(mname, "etc/dnf/dnf.conf"), "a")
                line_not_found = True
                for line in dnf_conf_contents:
                    if "[main]\n" == line:
                        line = line + "reposdir=" + repo_dest + "\n"
                        line_not_found = False
                        dnf_conf.write(line)
                        break
                ## Otherwise, add "[main]" and reposdir line
                if line_not_found:
                    dnf_conf.write("[main]\n" + "reposdir=" + repo_dest + "\n")

                dnf_conf.close()

        else:
            self.logger.error("unsupported package manager")

        inst = None
        try:
            inst = installer.Installer(package_manager, cname, mname, gpgcheck)
        except Exception as e:
            self.logger.error(f"Error preparing installer: {e}")
            cmd(["buildah","rm"] + [cname])
            sys.exit("Exiting now ...")
        except KeyboardInterrupt:
            self.logger.error(f"Keyboard Interrupt")
            cmd(["buildah","rm"] + [cname])
            sys.exit("Exiting now ...")

        # Install Repos
        try:
            if parent == "scratch":
                inst.install_scratch_repos(repos, repo_dest, proxy)
            else:
                inst.install_repos(repos, proxy)
        except Exception as e:
            self.logger.error(f"Error installing repos: {e}")
            cmd(["buildah","rm"] + [cname])
            sys.exit("Exiting now ...")
        except KeyboardInterrupt:
            self.logger.error(f"Keyboard Interrupt")
            cmd(["buildah","rm"] + [cname])
            sys.exit("Exiting now ...")

        # Install Packages
        try:
            if parent == "scratch":
                # Enable modules
                inst.install_scratch_modules(modules, repo_dest, self.args['proxy'])
                # Base Package Groups
                inst.install_scratch_package_groups(package_groups, repo_dest, proxy)
                # Packages
                inst.install_scratch_packages(packages, repo_dest, proxy)
            else:
                inst.install_package_groups(package_groups)
                inst.install_packages(packages)
            # Remove Packages
            inst.remove_packages(remove_packages)
        except Exception as e:
            self.logger.error(f"Error installing packages: {e}")
            cmd(["buildah","rm"] + [cname])
            sys.exit("Exiting now ...")
        except KeyboardInterrupt:
            self.logger.error(f"Keyboard Interrupt")
            cmd(["buildah","rm"] + [cname])
            sys.exit("Exiting now ...")

        # Run Commands
        try:
            inst.install_commands(commands)
            if os.path.islink(mname + '/etc/resolv.conf'):
                self.logger.info("removing resolv.conf link (this link breaks running a container)")
                os.unlink(mname + '/etc/resolv.conf')
        except Exception as e:
            self.logger.error(f"Error running commands: {e}")
            cmd(["buildah","rm"] + [cname])
            sys.exit("Exiting now")
        except KeyboardInterrupt:
            self.logger.error(f"Keyboard Interrupt")
            cmd(["buildah","rm"] + [cname])
            sys.exit("Exiting now ...")
        
        # OpenSCAP 
        if self.args['install_scap'] or self.args['scap_benchmark'] or self.args['oval_eval']: 
            oscap = Oscap(oscap_options, self.args, inst)
            if self.args['install_scap']:
                oscap.install_scap()
            oscap.check_install()
            if self.args['scap_benchmark']:
                oscap.run_oscap()
            if self.args['oval_eval']:
                oscap.run_oval_eval()

        return cname

    def _build_ansible(self, target, parent, ansible_groups, ansible_pb, ansible_inv, ansible_vars, ansible_verbosity):
        cnames = {}
        def buildah_handler(line):
            out.append(line)

        out = []
        cmd(["buildah","from"] + self.args['registry_opts_pull'] + ["--name", target, parent], stdout_handler = buildah_handler)
        container_name = out[0]

        cnames[container_name] = { 
                'ansible_groups': ansible_groups, 
                'ansible_pb': ansible_pb, 
                'ansible_vars': ansible_vars 
                }

        try:
            pb_res = run_playbook(cnames, ansible_inv, ansible_verbosity)
        except Exception as e:
            self.logger.error(e)
            cmd(["buildah","rm"] + [target])
            self.logger.error("Exiting Now...")
            sys.exit(1)
        return container_name

    def build_layer(self):
        print("BUILD LAYER".center(50, '-'))

        if self.args['layer_type'] == "base":
            
            repos = self.image_config.get_repos()
            modules = self.image_config.get_modules()
            packages = self.image_config.get_packages()
            package_groups = self.image_config.get_package_groups()
            remove_packages = self.image_config.get_remove_packages()
            commands = self.image_config.get_commands()
            copyfiles = self.image_config.get_copy_files()
            oscap_options = self.image_config.get_oscap_options()

            cname = self._build_base(repos, modules, packages, package_groups, remove_packages, commands, copyfiles, oscap_options)
        elif self.args['layer_type'] == "ansible":
            layer_name = self.args['name']
            print("Layer_Name =", layer_name)
            parent = self.args['parent']
            ansible_groups = self.args['ansible_groups']
            ansible_pb = self.args['ansible_pb']
            ansible_inv = self.args['ansible_inv']
            ansible_vars = self.args['ansible_vars']
            ansible_verbosity = self.args['ansible_verbosity']

            cname = self._build_ansible(layer_name, parent, ansible_groups, ansible_pb, ansible_inv, ansible_vars, ansible_verbosity)
        else:
            self.logger.error("Unrecognized layer type")
            sys.exit("Exiting now ...")
        
        # Publish the layer
        self.logger.info("Publishing Layer")
        publish(cname, self.args)
