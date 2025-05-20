from datetime import datetime
import sys
import os
# written modules
from image_config import ImageConfig
from utils import cmd, run_playbook
from publish import publish
import installer
import logging


class Layer:
    def __init__(self, args, image_config):
        self.args = args
        self.image_config = image_config
        self.logger = logging.getLogger(__name__)

    def _build_base(self, repos, modules, packages, package_groups, remove_packages, commands, copyfiles):
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
            repo_dest = "/etc/yum.repos.d"
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
                #inst.install_modules(modules, repo_dest, self.args['proxy'])
                inst.install_package_groups(package_groups, repo_dest, proxy)
                inst.install_packages(packages, repo_dest, proxy)
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

            cname = self._build_base(repos, modules, packages, package_groups, remove_packages, commands, copyfiles)
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
        
