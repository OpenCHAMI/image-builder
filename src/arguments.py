import os
import logging

def process_args(terminal_args, config_options):
    """
    Processes command line arguements and configuration options to generate a processed arguement dictionary 

    Returns: 
        dict: Processed arguements dictionary
    """
    processed_args = {}

    processed_args['log_level'] = terminal_args.log_level

    processed_args['config'] = terminal_args.config or "config.yaml"
    
    processed_args['layer_type'] = terminal_args.layer_type or config_options.get('layer_type')
    if not processed_args['layer_type']:
        raise ValueError("'layer_type' required in config file or as an arguement")

    if processed_args['layer_type'] == "base":
        processed_args['pkg_man'] = terminal_args.pkg_man or config_options.get('pkg_manager')
        if not processed_args['pkg_man']:
            raise ValueError("'pkg_man' required when 'layer_type' is base")
    elif processed_args['layer_type'] == "ansible":
        processed_args['ansible_groups'] = terminal_args.group_list or config_options.get('groups', [])
        processed_args['ansible_pb'] = terminal_args.pb or config_options.get('playbooks', [])
        processed_args['ansible_inv'] = terminal_args.inventory or config_options.get('inventory', [])
        processed_args['ansible_vars'] = terminal_args.inventory or config_options.get('vars', {})

        verbosity_values = list(range(0,5))
        if terminal_args.ansible_verbosity in verbosity_values:
            processed_args['ansible_verbosity'] = terminal_args.ansible_verbosity           
        else:
            # Handle invalid verbosity
            raise ValueError(
                """invalid ansible_verbosity: must be one of the following numerical values.
                       0 (default): Displays only critical information.
                       1 (-v):      Shows basic information like task names and results.
                       2 (-vv):     Includes more detailed output, such as variable values.
                       3 (-vvv):    Displays additional debugging data, such as task-level operations.
                       4 (-vvvv):   Enables connection debugging, providing a deep dive into network communication."""
            )
    processed_args['parent'] = terminal_args.parent or config_options.get('parent', 'scratch')
    processed_args['proxy'] = terminal_args.proxy or config_options.get('proxy', '')

    processed_args['name'] = terminal_args.name or config_options.get('name', 'base')

    processed_args['publish_local'] = terminal_args.publish_local or config_options.get('publish_local', False)

    processed_args['publish_s3'] = config_options.get('publish_s3', '') or terminal_args.publish_s3
    if processed_args['publish_s3']:
        processed_args['credentials'] = {
            'endpoint_url': processed_args['publish_s3'],
            'access_key': os.getenv('S3_ACCESS'),
            'secret_key': os.getenv('S3_SECRET')
        }
        processed_args['s3_prefix'] = terminal_args.s3_prefix or config_options.get('s3_prefix', '')
        processed_args['s3_bucket'] = terminal_args.s3_bucket or config_options.get('s3_bucket', 'boot-images')

    processed_args['publish_registry'] = config_options.get('publish_registry', '') or terminal_args.publish_registry
    if processed_args['publish_registry']:
        processed_args['registry_opts_push'] = terminal_args.registry_opts_push or config_options.get('registry_opts_push', [])
    processed_args['registry_opts_pull'] = terminal_args.registry_opts_pull or config_options.get('registry_opts_pull', [])

    processed_args['publish_tags'] = terminal_args.publish_tags or config_options.get('publish_tags',['latest'])
    
    processed_args['scap_benchmark'] = terminal_args.scap_benchmark or config_options.get('scap_benchmark', False)
    processed_args['oval_eval'] = terminal_args.oval_eval or config_options.get('oval_eval', False)
    processed_args['install_scap'] = terminal_args.install_scap or config_options.get('install_scap', False)

    # If no publish options were passed in either the CLI or the config file, store locally.
    if not (processed_args['publish_s3']
            or processed_args['publish_registry']
            or 'publish_local' in config_options.keys()
            or terminal_args.publish_local):
        logging.warn("No publish options passed, not storing image anywhere!")
        logging.warn("Use one or more of --publish-local, --publish-s3, or --publish-registry")
        logging.warn("to store in one or more of those locations.")

    return processed_args

def print_args(args):
    """
    Takes in a dictionary of arguements and prints them out
    """
    print()
    logging.info("ARGUEMENTS".center(50, '-'))

    for key, value in args.items():
        # do not print credentials to output
        if key == 'credentials':
            logging.info(f"s3 endpoint : {value['endpoint_url']}")
        elif key == 'registry':
            logging.info(f"registry endpoint : {value['endpoint']}")
        else:
            logging.info(f"{key} : {value}")
