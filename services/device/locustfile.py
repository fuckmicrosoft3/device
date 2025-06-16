"""
Locust file for load testing device message endpoint.
Simulates 1000 devices sending messages every 1 second.
"""
import json
import random
import time
from locust import HttpUser, task, between
from datetime import datetime

class DeviceUser(HttpUser):

    wait_time = between(0.1, 0.2)
    
    def on_start(self):
        """Load the devices file and pick a random device to simulate"""
        try:
            with open("devices.json", "r") as f:
                self.devices = json.load(f)
            
            if not self.devices:
                print("No devices found in devices.json. Run create_devices.py first.")
                self.environment.runner.quit()
                return
                
            # Pick a random device to simulate from the list
            self.device = random.choice(self.devices)
            self.device_uid = self.device["uid"]
            print(f"Simulating device: {self.device['serial']} (UID: {self.device_uid})")
        except Exception as e:
            print(f"Error loading devices.json: {e}")
            print("Make sure to run create_devices.py first to generate the devices file.")
            self.environment.runner.quit()
    
    def generate_message(self):
        """Generate a test device message"""

        ram_usage = random.randint(20, 95) 
        milliseconds = random.randint(1000000, 9999999)  
        seconds_up = random.randint(1, 86400)  
        memory_map = random.randint(10000, 99999)  
        
        # Create a message similar to the sample provided
        message = {
            "ev": "check",
            "rss": ram_usage,
            "ms": milliseconds,
            "s": seconds_up,
            "mm": memory_map
        }
        
        if random.random() < 0.3:
            message["ts"] = int(time.time())
        
        if random.random() < 0.2:
            message["temp"] = round(random.uniform(35.5, 85.0), 2)
        
        return message
    
    @task
    def send_device_message(self):
        """Send a device message to the API"""
        message = self.generate_message()
        

        payload = {
            "device_uid": self.device_uid,
            "message": json.dumps(message),
            "sent_via": "locust-test"
        }
        
        current_time = datetime.now().strftime("%Y-%m-%d %H:%M:%S")
        
        with self.client.post(
            "/api/v1/devices/messages",
            json=payload,
            catch_response=True
        ) as response:
            if response.status_code == 200:
                response.success()
            else:
                error_msg = f"Failed to send message: {response.status_code} - {response.text}"
                response.failure(error_msg)
                print(f"[{current_time}] Device {self.device['serial']}: {error_msg}")