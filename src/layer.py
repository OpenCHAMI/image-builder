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

    def buildah_handler(line):
        out.append(line)
        return out

    def _build_base(self, repos, packages, package_groups, remove_packages, commands, copyfiles):
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
            repo_dest = mname+"/etc/zypp/repos.d"
        elif self.args['pkg_man'] == "dnf":
            repo_dest = mname+"/etc/yum.repos.d"
        else:
            self.logger.error("unsupported package manager")

        # Install Repos
        try: 
            installer.install_repos(mname, cname, repos, repo_dest, self.args['pkg_man'], self.args['proxy'])
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
            # Base Package Groups
            installer.install_base_package_groups(cname, package_groups, repo_dest, mname, self.args['pkg_man'], self.args['proxy'])
            # Packages
            installer.install_base_packages(cname, packages, repo_dest, mname, self.args['pkg_man'], self.args['proxy'])
            # Remove Packages
            installer.remove_base_packages(cname, remove_packages)
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
            installer.install_base_copyfiles(cname, copyfiles)
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
            installer.install_base_commands(cname, commands)
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

    def _build_ansible(self, target, parent, ansible_groups, ansible_pb, ansible_inv, ansible_vars):
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
            pb_res = run_playbook(cnames, ansible_inv)
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
            packages = self.image_config.get_packages()
            package_groups = self.image_config.get_package_groups()
            remove_packages = self.image_config.get_remove_packages()
            commands = self.image_config.get_commands()
            copyfiles = self.image_config.get_copy_files()

            cname = self._build_base(repos, packages, package_groups, remove_packages, commands, copyfiles)
        elif self.args['layer_type'] == "ansible":
            layer_name = self.args['name']
            print("Layer_Name =", layer_name)
            parent = self.args['parent']
            ansible_groups = self.args['ansible_groups']
            ansible_pb = self.args['ansible_pb']
            ansible_inv = self.args['ansible_inv']
            ansible_vars = self.args['ansible_vars']

            cname = self._build_ansible(layer_name, parent, ansible_groups, ansible_pb, ansible_inv, ansible_vars)
        else:
            self.logger.error("Unrecognized layer type")
            sys.exit("Exiting now ...")
        
        publish(cname, self.args)
        