import logging
import os
import pathmod
import tempfile
from pathlib import Path
import urllib.request
import urllib.error

# Written Modules
from utils import cmd


class Installer:
    def __init__(self, pkg_man, cname, mname):
        self.pkg_man = pkg_man
        self.cname = cname
        self.mname = mname

        # Create temporary directory for logs, cache, etc. for package manager
        os.makedirs(os.path.join(mname, "tmp"), exist_ok=True)
        self.tdir = tempfile.mkdtemp(prefix="image-build-")
        logging.info(f'Installer: Temporary directory for {self.pkg_man} created at {self.tdir}')

        if pkg_man == "dnf":
            # DNF complains if the log directory is not present
            os.makedirs(os.path.join(self.tdir, "dnf/log"))

    def download_and_import_gpg_keys_from_repos(self, repos):
        """
        Download GPG keys from repos and import them into the RPM database.

        Args:
            repos: List of repo dicts, each may contain 'gpg_key' field

        Returns:
            True if all repos have gpg_key and all keys were successfully downloaded and imported
            False if any repo lacks gpg_key or any key failed to download or import
        """
        # Extract gpg_key URLs and validate all repos have them
        gpg_key_urls = []
        for repo in repos:
            alias = repo.get("alias", "unknown")
            if "gpg_key" not in repo:
                logging.info(f"Repo '{alias}' missing gpg_key field")
                return False
            gpg_key_urls.append(repo["gpg_key"])

        if not gpg_key_urls:
            logging.info("No GPG key URLs found in repos")
            return False

        # Deduplicate URLs
        unique_urls = list(set(gpg_key_urls))
        logging.info(
            f"Found {len(unique_urls)} unique GPG key(s) to download from {len(gpg_key_urls)} repos"
        )

        # Download and import each unique key
        success = True
        for idx, url in enumerate(unique_urls):
            key_filename = f"gpg-key-{idx + 1}.asc"
            key_path = os.path.join(self.tdir, key_filename)

            try:
                logging.info(f"Downloading GPG key from {url}")
                with urllib.request.urlopen(url, timeout=30) as response:
                    key_data = response.read()

                with open(key_path, "wb") as key_file:
                    key_file.write(key_data)

                logging.info(f"Importing GPG key: {key_filename}")
                import_cmd = ["rpm", "--root", self.mname, "--import", key_path]
                rc = cmd(import_cmd)

                if rc != 0:
                    logging.warning(f"Failed to import GPG key from {url}")
                    success = False
                else:
                    logging.info(f"Successfully imported GPG key from {url}")

            except urllib.error.URLError as e:
                logging.warning(f"Failed to download GPG key from {url}: {e}")
                success = False
            except Exception as e:
                logging.warning(f"Error processing GPG key from {url}: {e}")
                success = False

        return success

    def install_scratch_repos(self, repos, repo_dest):
        # check if there are repos passed for install
        if len(repos) == 0:
            logging.info("REPOS: no repos passed to install\n")
            self.gpg_keys_imported = False
            return

        # Try to download and import GPG keys from repos
        self.gpg_keys_imported = self.download_and_import_gpg_keys_from_repos(repos)

        if not self.gpg_keys_imported:
            logging.warning(
                "GPG keys not available - will use --nogpgcheck for package operations"
            )

        for r in repos:
            alias = r['alias']
            config = r['config'].lstrip()

            # Makes sure configs start with an alias
            if not config.startswith(f'['):
                config = f'[{alias}]\n{config}'

            repo_path = Path(*[self.mname, pathmod.sep_strip(repo_dest), f'{alias}.repo'])
            try:
                repo_path.parent.mkdir(parents=True, exist_ok=True)
                logging.info(f"REPOS: Writing {alias} to {repo_dest}")
                with open(repo_path, "w") as f:
                    f.write(config)
            except Exception as e:
                raise Exception(f"Failed to generate repo file for r['alias'] got:\n{e}")

    def install_scratch_packages(self, packages, registry_loc):
        # check if there are packages to install
        if len(packages) == 0:
            logging.warning("PACKAGES: no packages passed to install\n")
            return

        logging.info(f"PACKAGES: Installing these packages to {self.cname}")
        logging.info("\n".join(packages))

        # Check if GPG keys were successfully imported during repo setup
        use_nogpgcheck = not getattr(self, "gpg_keys_imported", False)
        if use_nogpgcheck:
            logging.warning("WARNING: Using --nogpgcheck for package installation. GPG keys were not available or failed to download.")

        args = []
        if self.pkg_man == "zypper":
            args.append("--non-interactive")
            args.append("-n")
            args.append("-D")
            args.append(os.path.join(self.mname, pathmod.sep_strip(registry_loc)))
            args.append("-C")
            args.append(self.tdir)
            args.append("--installroot")
            args.append(self.mname)
            args.append("install")
            args.append("-l")
            if use_nogpgcheck:
                args.append("--no-gpg-checks")
            args.extend(packages)
        elif self.pkg_man == "dnf":
            args.append("install")
            args.append("-y")
            if use_nogpgcheck:
                args.append("--nogpgcheck")
            args.append("--installroot")
            args.append(self.mname)
            args.extend(packages)

        rc = cmd([self.pkg_man] + args)
        if rc == 104:
            raise Exception("Installing base packages failed")

        if rc == 107:
            logging.warning("one or more RPM postscripts failed to run")

    def install_scratch_package_groups(self, package_groups):
        # check if there are packages groups to install
        if len(package_groups) == 0:
            logging.warning("PACKAGE GROUPS: no package groups passed to install\n")
            return

        logging.info(f"PACKAGE GROUPS: Installing these package groups to {self.cname}")
        logging.info("\n".join(package_groups))

        # Check if GPG keys were successfully imported during repo setup
        use_nogpgcheck = not getattr(self, "gpg_keys_imported", False)
        if use_nogpgcheck:
            logging.warning("Using --nogpgcheck for package group installation. GPG keys were not available or failed to download.")

        args = []

        if self.pkg_man == "zypper":
            logging.warning("zypper does not support package groups")
        elif self.pkg_man == "dnf":
            args.append("groupinstall")
            args.append("-y")
            if use_nogpgcheck:
                args.append("--nogpgcheck")
            args.append("--installroot")
            args.append(self.mname)
            args.extend(package_groups)

        rc = cmd([self.pkg_man] + args)
        if rc == 104:
            raise Exception("Installing base packages failed")

    def install_scratch_modules(self, modules):
        # check if there are modules groups to install
        if len(modules) == 0:
            logging.warning("PACKAGE MODULES: no modules passed to install\n")
            return
        logging.info(f"MODULES: Running these module commands for {self.cname}")
        for mod_cmd, mod_list in modules.items():
            logging.info(mod_cmd + ": " + " ".join(mod_list))

        # Check if GPG keys were successfully imported during repo setup
        use_nogpgcheck = not getattr(self, "gpg_keys_imported", False)
        if use_nogpgcheck:
            logging.warning("Using --nogpgcheck for module operations. GPG keys were not available or failed to download.")

        for mod_cmd, mod_list in modules.items():
            args = []
            if self.pkg_man == "zypper":
                logging.warning("zypper does not support package groups")
                return
            elif self.pkg_man == "dnf":
                args.append("module")
                args.append(mod_cmd)
                args.append("-y")
                if use_nogpgcheck:
                    args.append("--nogpgcheck")
                args.append("--installroot")
                args.append(self.mname)
                args.extend(mod_list)
            rc = cmd([self.pkg_man] + args)
            if rc != 0:
                raise Exception("Failed to run module cmd", mod_cmd, ' '.join(mod_list))

    def install_repos(self, repos, repo_dest):
        # check if there are repos passed for install
        if len(repos) == 0:
            logging.info("REPOS: no repos passed to install\n")
            self.gpg_keys_imported = False
            return

        # Try to download and import GPG keys from repos
        self.gpg_keys_imported = self.download_and_import_gpg_keys_from_repos(repos)

        if not self.gpg_keys_imported:
            logging.warning(
                "GPG keys not available - will use --nogpgcheck for package operations"
            )

        logging.info(f"REPOS: Installing these repos to {self.cname}")
        for r in repos:
            alias = r['alias']
            config = r['config'].lstrip()

            # Makes sure configs start with an alias
            if not config.startswith(f'['):
                config = f'[{alias}]\n{config}'

            with tempfile.NamedTemporaryFile(mode="w", delete=False) as tmp:
                tmp.write(config)
                tmp_path = tmp.name

            repo_path = Path(*[repo_dest, f'{alias}.repo'])
            copy_config = ['buildah', 'copy', self.cname, tmp_path, repo_path]
            mkparent_dir = ['buildah', 'run', self.cname, '--', 'mkdir', '-p', repo_dest]
            try:
                rc = cmd(mkparent_dir)
                if rc != 0:
                    raise Exception(f"Failed to create parent dir {repo_dest}")
                rc = cmd(copy_config)
                if rc != 0:
                    raise Exception(f"Failed to generate repo config at {repo_path}")
                logging.info(f"REPOS: Writing {alias} to {repo_path}")
            except Exception as e:
                raise Exception(f"Failed to generate repo file for r['alias'] got:\n{e}")
            finally:
                os.remove(tmp_path)

    def install_packages(self, packages):
        if len(packages) == 0:
            logging.warning("PACKAGE GROUPS: no package groups passed to install\n")
            return
        logging.info(f"PACKAGES: Installing these packages to {self.cname}")
        logging.info("\n".join(packages))
        args = [self.cname, '--', 'bash', '-c']
        pkg_cmd = [self.pkg_man]

        if self.gpgcheck is not True:
            if self.pkg_man == 'dnf':
                pkg_cmd.append('--nogpgcheck')
            elif self.pkg_man == 'zypper':
                pkg_cmd.append('--no-gpg-checks')
                pkg_cmd.append('--gpg-auto-import-keys')

        args.append(" ".join(pkg_cmd + ['install', '-y'] + packages))
        cmd(["buildah", "run"] + args)

    def install_package_groups(self, package_groups):
        if len(package_groups) == 0:
            logging.warning("PACKAGE GROUPS: no package groups passed to install\n")
            return
        logging.info(f"PACKAGES: Installing these package groups to {self.cname}")
        logging.info("\n".join(package_groups))
        args = [self.cname, '--', 'bash', '-c']
        pkg_cmd = [self.pkg_man, 'groupinstall', '-y']
        if self.pkg_man == "zypper":
            logging.warning("zypper does not support package groups")
        args.append(" ".join(pkg_cmd + [f'"{pg}"' for pg in package_groups]))
        cmd(["buildah", "run"] + args)

    def install_modules(self, modules):
        # check if there are modules groups to install
        if len(modules) == 0:
            logging.warning("PACKAGE MODULES: no modules passed to install\n")
            return
        logging.info(f"MODULES: Running these module commands for {self.cname}")
        args = [self.cname, '--', 'bash', '-c']
        pkg_cmd = [self.pkg_man]
        for mod_cmd, mod_list in modules.items():
            logging.info(mod_cmd + ": " + " ".join(mod_list))
        for mod_cmd, mod_list in modules.items():
            if self.pkg_man == "zypper":
                logging.warning("zypper does not support package groups")
                return
            elif self.pkg_man == "dnf":
                pkg_cmd.append("module")
                pkg_cmd.append(mod_cmd)
                pkg_cmd.append("-y")
                pkg_cmd.append("--nogpgcheck")
            args.append(" ".join(pkg_cmd + mod_list))
            cmd(["buildah", "run"] + args)

    def remove_packages(self, remove_packages):
        # check if there are packages to remove
        if len(remove_packages) == 0:
            logging.warning("REMOVE PACKAGES: no package passed to remove\n")
            return

        logging.info(f"REMOVE PACKAGES: removing these packages from container {self.cname}")
        logging.info("\n".join(remove_packages))
        for p in remove_packages:
            args = [self.cname, '--', 'rpm', '-e', '--nodeps', p]
            cmd(["buildah", "run"] + args)

    def install_commands(self, commands):
        # check if there are commands to install
        if len(commands) == 0:
            logging.warning("COMMANDS: no commands passed to run\n")
            return

        logging.info(f"COMMANDS: running these commands in {self.cname}")
        for c in commands:
            logging.info(c['cmd'])
            build_cmd = ['buildah', 'run']
            if "buildah_extra_args" in c:
                build_cmd.extend(c["buildah_extra_args"])
            args = [self.cname, '--', 'bash', '-c', c['cmd']]
            if 'loglevel' in c:
                if c['loglevel'].upper() == "INFO":
                    loglevel = logging.info
                elif c['loglevel'].upper() == "WARN":
                    loglevel = logging.warning
                else:
                    loglevel = logging.error
            else:
                loglevel = logging.error
            cmd(['buildah', 'run'] + args, stderr_handler=loglevel)

    def install_copyfiles(self, copyfiles):
        if len(copyfiles) == 0:
            logging.warning('COPYFILES: no files to copy\n')
            return
        logging.info(f"COPYFILES: copying these files to {self.cname}")
        for f in copyfiles:
            args = []
            if 'opts' in f:
                for o in f['opts']:
                    args.extend(o.split())
            logging.info(f['src'] + ' -> ' + f['dest'])
            args += [self.cname, f['src'], f['dest']]
            cmd(["buildah", "copy"] + args)
