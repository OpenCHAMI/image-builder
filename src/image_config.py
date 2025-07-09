"""
Image Configuration Module

This module provides a class, ImageConfig, for parsing YAML configuration files. 
It allows retrieving various options, packages, package groups, commands,
repositories, etc., specified in the YAML file.
"""

import yaml
import os

class ImageConfig:
    def __init__(self, yaml_file):
        if not os.path.exists(yaml_file):
            raise FileNotFoundError(f"The file '{yaml_file}' does not exist.")
        self.config_data = self._load_yaml(yaml_file)

    def _load_yaml(self, yaml_file):
        with open(yaml_file, 'r') as f:
            return yaml.safe_load(f)

    def get_options(self):
        return self.config_data.get('options', [])

    def get_modules(self):
        return self.config_data.get('modules', [])

    def get_packages(self):
        return self.config_data.get('packages', [])

    def get_package_groups(self):
        return self.config_data.get('package_groups', [])

    def get_remove_packages(self):
        return self.config_data.get('remove_packages', [])

    def get_commands(self):
        return self.config_data.get('cmds', [])

    def get_copy_files(self):
        return self.config_data.get('copyfiles', [])

    def get_repos(self):
        return self.config_data.get('repos', [])

    def get_oscap_options(self):
        return self.config_data.get('openscap', [])


if __name__ == "__main__":
    config = ImageConfig("ochami-images/base-configs/base.yaml")
    options = config.get_options()
    repos = config.get_repos()
    modules = config.get_modules()
    package_groups = config.get_package_groups()
    packages = config.get_packages()
    remove_packages = config.get_remove_packages()
    commands = config.get_commands()
    copyfiles = config.get_copy_files()
    oscap_options = config.get_oscap_options()

    print("Options: ", options)
    print("Repos: ", repos)
    print("Package Groups", package_groups)
    print("Packages: ", packages)
    print("Remove Packages: ", remove_packages)
    print("Commands: ", commands)
    print("OpenSCAP Options: ", oscap_options)
