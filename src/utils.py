from ansible import context
from ansible.cli import CLI
from ansible.module_utils.common.collections import ImmutableDict
from ansible.inventory.manager import InventoryManager
from ansible.parsing.dataloader import DataLoader
from ansible.executor.playbook_executor import PlaybookExecutor
from ansible.playbook import Playbook
from ansible.vars.manager import VariableManager
from ansible.config.manager import ConfigManager, Setting
from ansible.cli.config import ConfigCLI
from ansible.parsing.yaml.objects import AnsibleVaultEncryptedUnicode

try:
    from ansible.plugins.loader import init_plugin_loader
    plugin_loader_available = True
except ImportError:
    # Fallback for older Ansible versions where `init_plugin_loader` doesn't exist
    plugin_loader_available = False

import subprocess
import logging
from collections import deque
from concurrent.futures import ThreadPoolExecutor
from functools import partial
from subprocess import PIPE, CalledProcessError, CompletedProcess, Popen
import json
import yaml

def get_os(mdir):
    os_dict = {}
    os_version = ""
    with open(mdir+'/etc/os-release', 'r') as f:
        for line in f:
            if "=" in line:
                (k, v) = line.split('=')
                os_dict[k] = v

    if 'ID' in os_dict and 'VERSION_ID' in os_dict:
        os_version=os_dict['ID'].rstrip().replace('"','')+os_dict['VERSION_ID'].rstrip().replace('"','')
    elif 'ID_LIKE' in os_dict:
        os_version=os_dict['ID_LIKE'].rstrip().replace('"','')+'-'+os_dict['NAME'].rstrip().replace('"','')
    return os_version.lower()

#####
# Shamelessly stolen from https://stackoverflow.com/questions/18421757/live-output-from-subprocess-command/76634163#76634163
# User: ddelange
def cmd(
    args,
    *,
    stdout_handler=logging.info,
    stderr_handler=logging.error,
    check=True,
    text=True,
    stdout=PIPE,
    stderr=PIPE,
    **kwargs,
):
    with Popen(args, text=text, stdout=stdout, stderr=stderr, **kwargs) as process:
        with ThreadPoolExecutor(2) as pool:  # two threads to handle the streams
            exhaust = partial(pool.submit, partial(deque, maxlen=0))
            exhaust(stdout_handler(line[:-1]) for line in process.stdout)
            exhaust(stderr_handler(line[:-1]) for line in process.stderr)
    retcode = process.poll()
    if check and retcode:
        if retcode != 107:
            raise CalledProcessError(retcode, process.args)
    CompletedProcess(process.args, retcode)
    return retcode
   
def run_playbook(cnames, ansible_inv, ansible_verbosity):

    if plugin_loader_available:
        init_plugin_loader()

    loader = DataLoader()
    inventory = InventoryManager(loader=loader, sources=ansible_inv)

    for c, cdata in cnames.items():
        for g in cdata['ansible_groups']:
            inventory.add_group(g)
            inventory.add_host(host=c, group=g)
        inventory._inventory.set_variable(c, "ansible_connection", "buildah")
        if 'ansible_vars' in cdata:
            for k,v in cdata['ansible_vars'].items():
                inventory._inventory.set_variable(c, k, v)

    variable_manager = VariableManager(loader=loader, inventory=inventory)
    for c in cnames:
        logging.info("Vars for host " + c + ":")
        ansible_vars = variable_manager.get_vars(host=inventory.get_host(c))
        filtered = {k: v for k, v in ansible_vars.items() if type(v) is not AnsibleVaultEncryptedUnicode}
        ansible_vars.clear()
        ansible_vars.update(filtered)
        formatted_str = json.dumps(ansible_vars, indent=2, default=list)
        logging.info(formatted_str)

    pbs = []
    subset = []
    for c, cdata in cnames.items():
        subset.append(c)
        if cdata['ansible_pb'] not in pbs:
            pbs.append(cdata['ansible_pb'])

    context.CLIARGS = ImmutableDict(
      tags={},
      listtags=False,
      listtasks=False,
      listhosts=False,
      syntax=False,
      module_path=None,
      forks=100,
      private_key_file=None,
      verbosity=ansible_verbosity,
      check=False,
      start_at_task=None,
      subset=subset
      )

    CLI.get_host_list(inventory, context.CLIARGS['subset'])

    pbex = PlaybookExecutor(playbooks=pbs, inventory=inventory, variable_manager=variable_manager, loader=loader, passwords={})
    results = pbex.run()
    if results != 0:
        raise Exception("Ansible playbook failed to run ")
