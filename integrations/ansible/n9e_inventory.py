#!/usr/bin/env python3
# -*- coding: utf-8 -*-

"""
Nightingale Ansible Dynamic Inventory Script

This script generates a dynamic inventory for Ansible from Nightingale managed hosts.
It outputs JSON in the format expected by Ansible's dynamic inventory feature.

Usage:
    ansible-inventory -i n9e_inventory.py --list
    ansible-playbook -i n9e_inventory.py playbook.yml
"""

import json
import sys
import os
import argparse
import configparser

# Add the project root to the Python path so we can import our modules
sys.path.insert(0, os.path.join(os.path.dirname(__file__), '..', '..'))

try:
    import django
    # Set up Django settings
    os.environ.setdefault('DJANGO_SETTINGS_MODULE', 'nightingale.settings')
    django.setup()
    
    from django.db import connection
except ImportError:
    # If Django is not available, we'll use a different approach
    pass

def get_database_config():
    """Get database configuration from Nightingale config"""
    # Try to read from config file
    config_path = os.path.join(os.path.dirname(__file__), '..', '..', 'etc', 'config.yaml')
    if os.path.exists(config_path):
        import yaml
        with open(config_path, 'r') as f:
            config = yaml.safe_load(f)
            return config.get('db', {})
    
    # Try to read from environment variables
    return {
        'host': os.environ.get('DB_HOST', '127.0.0.1'),
        'port': int(os.environ.get('DB_PORT', 3306)),
        'user': os.environ.get('DB_USER', 'root'),
        'password': os.environ.get('DB_PASSWORD', ''),
        'database': os.environ.get('DB_NAME', 'nightingale'),
        'type': os.environ.get('DB_TYPE', 'mysql')
    }

def get_managed_hosts():
    """Get managed hosts from Nightingale database"""
    db_config = get_database_config()
    
    # For now, we'll return a sample structure
    # In a real implementation, this would connect to the database and query the managed_host table
    return [
        {
            'target_ident': 'host1',
            'ssh_ip': '192.168.1.10',
            'ssh_port': 22,
            'ssh_user': 'root',
            'auth_method': 'key',
            'status': 'active',
            'sudo_required': False
        },
        {
            'target_ident': 'host2',
            'ssh_ip': '192.168.1.11',
            'ssh_port': 22,
            'ssh_user': 'admin',
            'auth_method': 'password',
            'status': 'pending',
            'sudo_required': True
        }
    ]

def get_host_credentials(target_ident, auth_method):
    """Get host credentials from Nightingale config store"""
    # For now, we'll return sample credentials
    # In a real implementation, this would query the configs table
    if auth_method == 'key':
        return "-----BEGIN OPENSSH PRIVATE KEY-----
...
-----END OPENSSH PRIVATE KEY-----"
    elif auth_method == 'password':
        return "sample_password"
    return ""

def generate_inventory():
    """Generate Ansible dynamic inventory"""
    hosts = get_managed_hosts()
    
    inventory = {
        '_meta': {
            'hostvars': {}
        },
        'all': {
            'children': ['managed_hosts']
        },
        'managed_hosts': {
            'hosts': []
        }
    }
    
    # Create temporary directory for keys if needed
    temp_key_dir = '/tmp/n9e_ansible_keys'
    if not os.path.exists(temp_key_dir):
        os.makedirs(temp_key_dir)
    
    for host in hosts:
        target_ident = host['target_ident']
        ssh_ip = host['ssh_ip']
        ssh_port = host['ssh_port']
        ssh_user = host['ssh_user']
        auth_method = host['auth_method']
        sudo_required = host['sudo_required']
        
        # Add to hosts list
        inventory['managed_hosts']['hosts'].append(target_ident)
        
        # Set host variables
        inventory['_meta']['hostvars'][target_ident] = {
            'ansible_host': ssh_ip,
            'ansible_port': ssh_port,
            'ansible_user': ssh_user,
        }
        
        # Add authentication details
        if auth_method == 'key':
            # In a real implementation, we would create a temporary key file
            # and set ansible_ssh_private_key_file
            # For now, we'll just add a placeholder
            inventory['_meta']['hostvars'][target_ident]['ansible_ssh_private_key_file'] = f'/tmp/n9e_ansible_keys/{target_ident}'
        elif auth_method == 'password':
            # For password authentication, we would typically use ansible-vault
            # or a temporary file. For now, we'll just add a placeholder.
            # Note: This is not secure and should be handled with ansible-vault in production
            inventory['_meta']['hostvars'][target_ident]['ansible_ssh_pass'] = '{{ vault_ssh_password_' + target_ident + ' }}'
        
        # Add sudo requirement
        if sudo_required:
            inventory['_meta']['hostvars'][target_ident]['ansible_become'] = True
            inventory['_meta']['hostvars'][target_ident]['ansible_become_method'] = 'sudo'
    
    return inventory

def main():
    """Main function"""
    parser = argparse.ArgumentParser(description='Nightingale Ansible Dynamic Inventory')
    parser.add_argument('--list', action='store_true', help='List all hosts')
    parser.add_argument('--host', help='Get variables for a specific host')
    
    args = parser.parse_args()
    
    if args.host:
        # Return variables for a specific host
        # In a real implementation, we would query for this specific host
        inventory = generate_inventory()
        hostvars = inventory['_meta']['hostvars'].get(args.host, {})
        print(json.dumps(hostvars, indent=2))
    else:
        # Return the full inventory
        inventory = generate_inventory()
        print(json.dumps(inventory, indent=2))

if __name__ == '__main__':
    main()