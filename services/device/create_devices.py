#!/usr/bin/env python3
import requests
import json
import uuid
import time
import sys
import argparse

def create_organization(base_url, org_name="Load Test Organization"):
    """Create an organization for the test devices"""
    url = f"{base_url}/api/v1/organizations"
    payload = {
        "name": org_name,
        "uri": "load-test-org",
        "persist": True
    }
    
    response = requests.post(url, json=payload)
    if response.status_code not in (200, 201):
        print(f"Failed to create organization: {response.status_code} - {response.text}")
        sys.exit(1)
    
    org_id = response.json()["id"]
    print(f"Created organization with ID: {org_id}")
    return org_id

def create_devices(base_url, org_id, num_devices=1000, output_file="devices.json"):
    """Create the specified number of devices and save their IDs and UIDs to a file"""
    url = f"{base_url}/api/v1/devices"
    devices = []
    
    print(f"Creating {num_devices} devices...")
    
    for i in range(num_devices):
        # Generate a unique device UID
        device_uid = str(uuid.uuid4())
        serial = f"LOADTEST{i:04d}"
        
        payload = {
            "uid": device_uid,
            "serial": serial,
            "organization_id": org_id,
            "allow_updates": True,
            "active": True
        }
        
        response = requests.post(url, json=payload)
        
        if response.status_code not in (200, 201):
            print(f"Failed to create device {i+1}: {response.status_code} - {response.text}")
            continue
        
        device_data = response.json()
        devices.append({
            "id": device_data["id"],
            "uid": device_data["uid"],
            "serial": serial
        })
        
        # Print progress every 100 devices
        if (i + 1) % 100 == 0:
            print(f"Created {i+1} devices")
        
        # Add a slight delay to prevent overwhelming the API
        time.sleep(0.1)
    
    # Save devices to file
    with open(output_file, 'w') as f:
        json.dump(devices, f, indent=2)
    
    print(f"Created {len(devices)} devices successfully. Device data saved to {output_file}")
    return devices

def main():
    parser = argparse.ArgumentParser(description="Create devices for load testing")
    parser.add_argument("--url", default="http://localhost:8091", help="Base URL of the device service")
    parser.add_argument("--num", type=int, default=1000, help="Number of devices to create")
    parser.add_argument("--org", type=int, help="Organization ID to use (if not provided, a new org will be created)")
    parser.add_argument("--output", default="devices.json", help="Output file to save device data")
    
    args = parser.parse_args()
    
    print(f"Setting up {args.num} devices for load testing...")
    
    # Create or use organization
    org_id = args.org if args.org else create_organization(args.url)
    
    # Create devices
    create_devices(args.url, org_id, args.num, args.output)
    
    print("Setup complete! You can now run the Locust file for load testing.")

if __name__ == "__main__":
    main()