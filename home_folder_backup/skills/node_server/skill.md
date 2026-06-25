[DESCRIPTION]
Load when the user wants Node-RED, MQTT, Zigbee, or smart-home actions. Use for SSH access, MQTT control, reading room/outdoor temperatures, humidity, and energy/power consumption from smart plugs and sensors on the Node server. Server credentials, MQTT config, and full device list in DATA.

[BEHAVIOR]
Use this skill to interact with the Node smart-home server via SSH and MQTT.

## SSH access method
```bash
sshpass -p '<password>' ssh -o StrictHostKeyChecking=no node@<server_ip> '<command>'
```
`sshpass` must be installed locally. For IP and password, see `global/my-network` skill. MQTT topics and device addresses are in the DATA section below.

## MQTT commands

**Important:** `mosquitto-clients` is NOT installed locally. All MQTT commands MUST be run via SSH on the Node server. Do NOT attempt to run `mosquitto_sub`/`mosquitto_pub` directly on the local machine. Use the SSH pattern below.

**Always use friendly names, not raw addresses, in MQTT topics.** The config at `/opt/zigbee2mqtt/data/configuration.yaml` maps addresses like `0x00124b0025037c4c` to names like `Temperature_02`. The topic becomes `zigbee2mqtt/Temperature_02`. Using the raw address directly will timeout because no message is published under that topic.

**Battery-powered sensors** (temperature, humidity, PIR, smoke, water leak, button): Do NOT respond to on-demand reads. They only publish at their own interval (minutes to hours). Data is retained on the broker (`retain: true`), so you get the last reading instantly — no need to wait and no retry will produce a newer reading faster.

**Mains-powered devices** (smart plugs, lights, switches): Usually respond immediately BUT may appear offline if they haven't published recently. If a direct subscribe times out, use the zigbee2mqtt bridge refresh to wake them up (see fallback below).

### Device read fallback strategy (mains-powered devices)
1. Try direct subscribe first (5s timeout):
   ```bash
   mosquitto_sub -t "zigbee2mqtt/<friendly_name>" -C 1 -W 5 -v
   ```
2. If that times out, send a get request and retry:
   ```bash
   mosquitto_pub -t "zigbee2mqtt/<friendly_name>/get/state" -m ""
   sleep 2
   mosquitto_sub -t "zigbee2mqtt/<friendly_name>" -C 1 -W 5 -v
   ```
3. If still no response, **force a bridge refresh** (this wakes up sleeping mains devices):
   ```bash
   # First start a subscriber in the background
   timeout 8 mosquitto_sub -t "zigbee2mqtt/<friendly_name>" -v &
   # Then send the refresh command to the bridge
   mosquitto_pub -t "zigbee2mqtt/bridge/request/device/refresh" -m '{"id":"<friendly_name>"}'
   wait
   ```
   The bridge refresh forces zigbee2mqtt to poll the device. The response arrives within 1-3 seconds.

### Read a device state
```bash
sshpass -p '<password>' ssh -o StrictHostKeyChecking=no node@<server_ip> \
  'mosquitto_sub -t "zigbee2mqtt/<friendly_name>" -C 1 -W 5 -v'
```

### Control a device (ON/OFF)
```bash
sshpass -p '<password>' ssh -o StrictHostKeyChecking=no node@<server_ip> \
  'mosquitto_pub -t "zigbee2mqtt/<friendly_name>/set" -m "{\"state\": \"ON\"}"'
sshpass -p '<password>' ssh -o StrictHostKeyChecking=no node@<server_ip> \
  'mosquitto_pub -t "zigbee2mqtt/<friendly_name>/set" -m "{\"state\": \"OFF\"}"'
```
**Note:** Use `-r` (retain) only when you want the state to persist across restarts. For one-off commands, omit `-r`.

### Read temperature/humidity sensor
```bash
sshpass -p '<password>' ssh -o StrictHostKeyChecking=no node@<server_ip> \
  'mosquitto_sub -t "zigbee2mqtt/<friendly_name>" -C 1 -W 5 -v'
# Returns JSON with temperature, humidity, battery, etc.
```

**Note:** Temperature/humidity sensors are battery-powered. They don't respond to on-demand reads — they only publish at their own interval. Data is retained, so you always get the last reading. If the topic has never published, it will timeout — do not retry with a refresh, just report offline.

### Read power meter (smart plugs)
```bash
sshpass -p '<password>' ssh -o StrictHostKeyChecking=no node@<server_ip> \
  'mosquitto_sub -t "zigbee2mqtt/<friendly_name>" -C 1 -W 5 -v'
# Returns JSON with power, current, voltage, energy, state
# If it times out, use the bridge refresh fallback above before giving up.
```

## Tasmota devices

### Tasmota MQTT topic pattern
```
cmnd/<topic>/<command>    — send command  (e.g. POWER, POWER1, STATE)
stat/<topic>/<result>     — status response
tele/<topic>/<sensor>     — telemetry / sensor readings
tele/<topic>/LWT          — Last Will (Online/Offline)
```

### Control a Tasmota device (relay ON/OFF)
```bash
sshpass -p '<password>' ssh -o StrictHostKeyChecking=no node@<server_ip> \
  'mosquitto_pub -t "cmnd/<topic>/POWER" -m "ON"'
sshpass -p '<password>' ssh -o StrictHostKeyChecking=no node@<server_ip> \
  'mosquitto_pub -t "cmnd/<topic>/POWER" -m "OFF"'
# For multi-relay: POWER1, POWER2, etc.
```

### Read Tasmota sensor data
```bash
sshpass -p '<password>' ssh -o StrictHostKeyChecking=no node@<server_ip> \
  'mosquitto_sub -t "tele/<topic>/SENSOR" -C 1 -W 5 -v'
```

### Get Tasmota device status
```bash
sshpass -p '<password>' ssh -o StrictHostKeyChecking=no node@<server_ip> \
  'mosquitto_sub -t "stat/<topic>/STATUS" -C 1 -W 5 -v'
# Or request fresh status:
sshpass -p '<password>' ssh -o StrictHostKeyChecking=no node@<server_ip> \
  'mosquitto_pub -t "cmnd/<topic>/STATUS" -m ""'
```

### Read Tasmota LWT (connection state)
```bash
sshpass -p '<password>' ssh -o StrictHostKeyChecking=no node@<server_ip> \
  'mosquitto_sub -t "tele/<topic>/LWT" -C 1 -W 5 -v'
```

### Tasmota devices in this network
- **VentilatorRecuperare** — `tasmota_DBAE77` — 192.168.0.101 — 58:BF:25:DB:AE:77 — 13.1.0 — ESP8266EX — PMS5003
- **Purificator_Aer** — `tasmota_5066D9` — 192.168.0.107 — 40:91:51:50:66:D9 — 10.1.0 — ESP — PMS5003
- **Energy_Main** — `tasmota_1469A5` — 192.168.0.109 — 94:B9:7E:14:69:A5 — 10.1.0 — ESP — COUNTER

### Tasmota usage examples
```bash
# Turn OFF ventilator (via node server)
sshpass -p 'gheorghe' ssh -o StrictHostKeyChecking=no node@192.168.0.110 \
  'mosquitto_pub -t "cmnd/tasmota_DBAE77/POWER" -m "OFF"'

# Read PM sensor from Purificator_Aer (via node server)
sshpass -p 'gheorghe' ssh -o StrictHostKeyChecking=no node@192.168.0.110 \
  'mosquitto_sub -t "tele/tasmota_5066D9/SENSOR" -C 1 -W 5 -v'

# Read energy counter from Energy_Main (via node server)
sshpass -p 'gheorghe' ssh -o StrictHostKeyChecking=no node@192.168.0.110 \
  'mosquitto_sub -t "tele/tasmota_1469A5/SENSOR" -C 1 -W 5 -v'
```

### Tasmota device types (for reference)
- **Relay (POWER)** — ON/OFF control
- **PMS5003** — PM1, PM2.5, PM10 particulate sensor
- **COUNTER** — pulse counter (energy metering)

## Device types (for reference when interpreting data)
- **Smart plug** — ON/OFF + power metering (power, current, voltage, energy)
- **Light** — ON/OFF + brightness/color if configured
- **Temp/humidity sensor** — temperature, humidity, battery
- **Motion sensor (PIR)** — occupancy, battery
- **Smoke detector** — smoke, battery
- **Switch** — action/click type
- **Button** — single/double/long press
- **Water leak sensor** — water, battery
- **Heating/thermostat** — temperature, heating state, setpoint
- **Energy monitor** — total energy consumption
- **Ventilator recuperare** — ventilation control (Tasmota)
- **Repeater** — Zigbee signal repeater (no direct controls)

[DATA]
node.mqtt.base_topic=zigbee2mqtt
node.zigbee.config_path=/opt/zigbee2mqtt/data/configuration.yaml
node.url.zigbee_frontend=http://192.168.0.110:8080/
node.url.nodered=http://192.168.0.110:1880/
device.GeneralEnergy.address=0xa4c13869e61a8864
device.GeneralEnergy.type=powermonitor
device.Priza_Lidl_01.address=0x5c0272fffe854241
device.Priza_Lidl_01.type=smartplug
device.Priza_Lidl_01.description=Filtru osmoza
device.PIR_Lidl.address=0x847127fffea497f4
device.PIR_Lidl.type=motion
device.Plafon_Etaj.address=0x00124b0024c0de9a
device.Plafon_Etaj.type=light
device.Smoke_Birou.address=0xa4c1381e44469f4b
device.Smoke_Birou.type=smoke
device.Switch_2pos.address=0x04cd15fffe856954
device.Switch_2pos.type=switch
device.Bec_Ikea_1050lm_01.address=0x2c1165fffe64adbe
device.Bec_Ikea_1050lm_01.type=light
device.Switch_4pos.address=0xb4e3f9fffeacd285
device.Switch_4pos.type=switch
device.Priza_mon_03.address=0xa4c13802bb4251dc
device.Priza_mon_03.type=smartplug
device.Temperature_03.address=0xa4c138e94dee1784
device.Temperature_03.type=temp_humidity
device.Temperature_03.description=Birou
device.Temperature_04.address=0xa4c138a9c6f0d8ae
device.Temperature_04.type=temp_humidity
device.Temperature_04.description=Birou Etaj
device.Priza_Lidl_02.address=0xb4e3f9fffe78050a
device.Priza_Lidl_02.type=smartplug
device.Priza_Lidl_02.description=Router TV Etaj
device.Priza_PC.address=0xa4c1385c892d6196
device.Priza_PC.type=smartplug
device.Temperature_02.address=0x00124b0025037c4c
device.Temperature_02.type=temp_humidity
device.Temperature_02.description=Outdoor
device.Bec_Ikea_470lm_01.address=0x2c1165fffe26b5be
device.Bec_Ikea_470lm_01.type=light
device.Ikea_GU10_left.address=0x0c4314fffeb0f63c
device.Ikea_GU10_left.type=light
device.Ikea_GU10_right.address=0x0c4314fffeaeae03
device.Ikea_GU10_right.type=light
device.Priza_Boiler.address=0xa4c1383817b3ef44
device.Priza_Boiler.type=smartplug
device.Priza_mon_04.address=0xa4c1384abbb656a2
device.Priza_mon_04.type=smartplug
device.Priza_mon_04.description=Cafetiera
device.AC_Etaj.address=0xa4c138f0b70f56a3
device.AC_Etaj.type=ac
device.AC_Etaj.power_plug=AC_Etaj
device.Dormitor_Plafon.address=0xa4c1388d0708afe4
device.Dormitor_Plafon.type=light
device.Priza_mon_05.address=0xa4c13883b64ea066
device.Priza_mon_05.type=smartplug
device.Priza_mon_05.description=Priza portocalie. POMPA GRADINA
device.Smoke_Dormitor.address=0xa4c13890543ed900
device.Smoke_Dormitor.type=smoke
device.Repetor.address=0x00124b001cd44cbb
device.Repetor.type=repeater
device.Temperature_05.address=0xa4c13819cd747c2b
device.Temperature_05.type=temp_humidity
device.Temperature_05.description=Rulota
device.Switch_modif.address=0xa4c1389a78d5348c
device.Switch_modif.type=switch
device.Inundatie_Bucatarie.address=0xa4c1381025cbfe27
device.Inundatie_Bucatarie.type=waterleak
device.VentRec_02.address=0xa4c1388f7761c0c1
device.VentRec_02.type=ventilator
device.VentRec_02.description=Ventilator recuperare 02
device.VentRec_01.address=0xa4c13814646648b6
device.VentRec_01.type=ventilator
device.VentRec_01.description=Ventilator recuperare 01
device.Temperature_01.address=0x00124b00250073f4
device.Temperature_01.type=temp_humidity
device.Temperature_01.description=Dormitor
device.SingleButton_01.address=0xa4c138ad9e436e63
device.SingleButton_01.type=button
device.Inundatie_baie.address=0xa4c13894d35024ea
device.Inundatie_baie.type=waterleak
device.HeatingDormAlbert.address=0xa4c1388504f5478e
device.HeatingDormAlbert.type=heating
device.HeatingDormAlbert.description=AC Dorm Albert (Gree)
device.Heating_Dorm_NoMon.address=0xa4c13885fb833835
device.Heating_Dorm_NoMon.type=heating
device.Priza_mon_07.address=0xa4c138b4d2fa9c7e
device.Priza_mon_07.type=smartplug
device.Priza_mon_07.description=Priza frigider parter
device.Priza_mon_08.address=0xa4c138272e3490ad
device.Priza_mon_08.type=smartplug
device.Priza_mon_08.description=AC Birou Etaj (Gree)
device.Device_Energy_Monitor.address=0xa4c138fde562fc1d
device.Device_Energy_Monitor.type=energymonitor
device.Priza_mon_09.address=0x180df9fffe2143b8
device.Priza_mon_09.type=smartplug
device.Priza_mon_09.description=AC Parter 01
device.Priza_mon_10.address=0x180df9fffe1dea94
device.Priza_mon_10.type=smartplug
device.Temperature_06.address=0x00124b00292d766d
device.Temperature_06.type=temp_humidity
device.Temperature_06.description=Birou Etaj
device.Priza_mon_11.address=0x348d13fffe07e0cd
device.Priza_mon_11.type=smartplug
device.Priza_mon_12.address=0x348d13fffec8dd93
device.Priza_mon_12.type=smartplug
device.Priza_mon_12.description=Uscator _Etaj
device.VentilatorRecuperare.ip=192.168.0.101
device.VentilatorRecuperare.mac=58:BF:25:DB:AE:77
device.VentilatorRecuperare.type=tasmota
device.VentilatorRecuperare.topic=tasmota_DBAE77
device.VentilatorRecuperare.hostname=tasmota-DBAE77-3703
device.VentilatorRecuperare.firmware=13.1.0
device.VentilatorRecuperare.hardware=ESP8266EX
device.VentilatorRecuperare.sensors=PMS5003
device.VentilatorRecuperare.relays=2
device.Purificator_Aer.ip=192.168.0.107
device.Purificator_Aer.mac=40:91:51:50:66:D9
device.Purificator_Aer.type=tasmota
device.Purificator_Aer.topic=tasmota_5066D9
device.Purificator_Aer.hostname=tasmota-5066D9-1753
device.Purificator_Aer.firmware=10.1.0
device.Purificator_Aer.hardware=ESP
device.Purificator_Aer.sensors=PMS5003
device.Purificator_Aer.relays=0
device.Energy_Main.ip=192.168.0.109
device.Energy_Main.mac=94:B9:7E:14:69:A5
device.Energy_Main.type=tasmota
device.Energy_Main.topic=tasmota_1469A5
device.Energy_Main.hostname=tasmota-1469A5-2469
device.Energy_Main.firmware=10.1.0
device.Energy_Main.hardware=ESP
device.Energy_Main.sensors=COUNTER
device.Energy_Main.relays=0
device.RadonEye.address=30:C6:F7:BF:A4:FA
device.RadonEye.type=radon
device.RadonEye.model=RadonEye RD200
device.RadonEye.protocol=BLE
device.RadonEye.server=192.168.0.104
device.RadonEye.script=/home/nas/radon/radon_reader.py
device.RadonEye.cron=*/5 * * * * (root)
device.RadonEye.args=-v -a 30:C6:F7:BF:A4:FA -t 1 -b -s
device.RadonEye.udp_dest=192.168.0.110:5005
device.RadonEye.nodered_measurements=Radon_Eye,RDNPLSN,RDNPLSL
