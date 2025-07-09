import logging
from utils import cmd

class Oscap:
    def __init__(self, args, inst):
        self.args = args
        self.inst = inst
        self.logger = logging.getLogger(__name__)

    def check_install(self):
        check_install = self._check_scap_install() 
        commands = [
            {'cmd': check_install, 'loglevel': 'DEBUG'},
        ]
        try:
            self.inst.install_base_commands(commands)
        except Exception as e:
            self.logger.error(f"openscap not found - Try installing the following: openscap-utils scap-security-guide: {e}")
            cmd(["buildah","rm"] + [cname])
            sys.exit("Exiting now")
        except KeyboardInterrupt:
            self.logger.error(f"Keyboard Interrupt")
            cmd(["buildah","rm"] + [cname])
            sys.exit("Exiting now ...") 

    def install_scap(self):
        if self.args['pkg_man'] == "zypper":
            repo_dest = "/etc/zypp/repos.d"
        elif self.args['pkg_man'] == "dnf":
            repo_dest = "/etc/yum.repos.d"
        else:
            self.logger.error("unsupported package manager")
        scap_packages = self._generate_scap_package_list()
        try:
            self.inst.install_base_packages(scap_packages, repo_dest, self.args['proxy'])
        except Exception as e:
            self.logger.error(f"Issues installing openscap with repos available - typically available via distro appstream repo: {e}")
            cmd(["buildah","rm"] + [cname])
            sys.exit("Exiting now")
        except KeyboardInterrupt:
            self.logger.error(f"Keyboard Interrupt")
            cmd(["buildah","rm"] + [cname])
            sys.exit("Exiting now ...")


    def run_oval_eval(self, oscap_config):
        config = self._get_oscap_config()
        for i in oscap_config:
            config.update(i)
        self.logger.error(config)

        obtain_oval_cmd = self._generate_obtain_oval_cmd(config)
        evaluation_oval = self._generate_evaluate_oval(config)

        commands = [
            {'cmd': obtain_oval_cmd, 'loglevel': 'DEBUG'},
            {'cmd': evaluation_oval, 'loglevel': 'DEBUG'}
        ]
        try:
            self.inst.install_base_commands(commands)
        except Exception as e:
            self.logger.error("Error with SCAP oval eval - Please check the logs")
            cmd(["buildah","rm"] + [cname])
            sys.exit("Exiting now")
        except KeyboardInterrupt:
            self.logger.error(f"Keyboard Interrupt")
            cmd(["buildah","rm"] + [cname])
            sys.exit("Exiting now ...")

    def run_oscap(self, oscap_config):
        config = self._get_oscap_config()
        for i in oscap_config:
            config.update(i)
        self.logger.error(config)

        evaluation_cmd = self._generate_evaluate_cmd(config)
        remediation_cmd = self._generate_remediate_cmd(config)

        commands = [
            {'cmd': evaluation_cmd, 'loglevel': 'DEBUG'},
            {'cmd': remediation_cmd, 'loglevel': 'DEBUG'}
        ]
        try:
            self.inst.install_base_commands(commands)
        except Exception as e:
            self.logger.error("Error with SCAP benchmark and generation of remediation script - please check the logs")
            cmd(["buildah","rm"] + [cname])
            sys.exit("Exiting now")
        except KeyboardInterrupt:
            self.logger.error(f"Keyboard Interrupt")
            cmd(["buildah","rm"] + [cname])
            sys.exit("Exiting now ...")


    def _get_oscap_config(self):
        return {
            'results_path': "/root/scan.xml",
            'remediate_path': "/root/remediate.sh",
            'oval_xml': "/root/oval.xml",
            'oval_path': "/root/vulnerabilities.xml",
        }

    def _check_scap_install(self):
        return f"oscap --help"

    def _generate_scap_package_list(self):
        return ["openscap-utils",  "scap-security-guide", "bzip2"]

    def _generate_evaluate_cmd(self, config):
        return f"oscap xccdf eval --fetch-remote-resources --profile {config['profile']} --results {config['results_path']} {config['benchmark_path']} || true"

    def _generate_evaluate_oval(self, config):
        return f"oscap oval eval --report {config['oval_path']} {config['oval_xml']} || true"

    def _generate_obtain_oval_cmd(self, config):
        return f"curl -L -o - {config['oval_url']} | bzip2 --decompress > {config['oval_xml']}"

    def _generate_remediate_cmd(self, config):
        return f"oscap xccdf generate fix --output {config['remediate_path']} --profile {config['profile']} {config['results_path']}"

