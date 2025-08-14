from datetime import datetime
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

    def buildah_handler(line):
        out.append(line)
        return out

    def _build_base(self, repos, modules, packages, package_groups, remove_packages, commands, copyfiles, oscap_options):
        dt_string = datetime.now().strftime("%Y%m%d%H%M%S")

        # container and mount name
        def buildah_handler(line):
            out.append(line)

        out = []
        cmd(["buildah", "from"] + self.args['registry_opts_pull'] + ["--name", self.args['name']+ dt_string, self.args['parent']], stdout_handler = buildah_handler)
        cname = out[0]

        out = []
        cmd(["buildah", "mount"] + [cname], stdout_handler = buildah_handler)
        mname = out[0]

        self.logger.info(f"Container: {cname} mounted at {mname}")

        if self.args['pkg_man'] == "zypper":
            repo_dest = "/etc/zypp/repos.d"
        elif self.args['pkg_man'] == "dnf":
            repo_dest = "/etc/yum.repos.d"
        else:
            self.logger.error("unsupported package manager")

        inst = None
        try:
            inst = installer.Installer(self.args['pkg_man'], cname, mname)
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
            inst.install_repos(repos, repo_dest, self.args['proxy'])
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
            # Enable modules
            inst.install_base_modules(modules, repo_dest, self.args['proxy'])
            # Base Package Groups
            inst.install_base_package_groups(package_groups, repo_dest, self.args['proxy'])
            # Packages
            inst.install_base_packages(packages, repo_dest, self.args['proxy'])
            # Remove Packages
            inst.remove_base_packages(remove_packages)
        except Exception as e:
            self.logger.error(f"Error installing packages: {e}")
            cmd(["buildah","rm"] + [cname])
            sys.exit("Exiting now ...")
        except KeyboardInterrupt:
            self.logger.error(f"Keyboard Interrupt")
            cmd(["buildah","rm"] + [cname])
            sys.exit("Exiting now ...")

        # Copy Files
        try:
            inst.install_base_copyfiles(copyfiles)
        except Exception as e:
            self.logger.error(f"Error running commands: {e}")
            cmd(["buildah","rm"] + [cname])
            sys.exit("Exiting now")
        except KeyboardInterrupt:
            self.logger.error(f"Keyboard Interrupt")
            cmd(["buildah","rm"] + [cname])
            sys.exit("Exiting now ...")

        # Run Commands
        try:
            inst.install_base_commands(commands)
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
        
