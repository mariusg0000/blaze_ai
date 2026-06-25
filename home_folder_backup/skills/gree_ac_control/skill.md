[DESCRIPTION]
Load when the user wants to control, monitor, or configure Gree AC units. Use for power on/off, temperature setting, mode switching, fan speed, and reading AC status. Device credentials and network info in DATA.

[BEHAVIOR]
## Purpose
Control Gree HVAC units in the local network via UDP using the gree.py CLI script.

## Related Skill
- `global/my-network` contains server IPs and SSH credentials when network discovery or connectivity info is needed.

## Script Location
- The CLI script is at `{SKILL_DIR}/scripts/gree.py`
- Dependencies: `pycryptodome`, `cryptography` (preinstalled)

## Preferred Workflow
1. Read device credentials from the active DATA section below.
2. Use the gree.py script with the correct device IP, ID, key, and encryption type from DATA.
3. Always specify `-e GCM` since both ACs use GCM encryption.
4. For status queries, use the `get` command with parameter names.
5. For changes, use the `set` command with `Param=Value` pairs.

## Common Commands
- Read status: `python3 gree.py get Pow Mod SetTem TemSen -c <ip> -i <id> -k <key> -e GCM`
- Power on: `python3 gree.py set Pow=1 -c <ip> -i <id> -k <key> -e GCM`
- Power off: `python3 gree.py set Pow=0 -c <ip> -i <id> -k <key> -e GCM`
- Set temp: `python3 gree.py set SetTem=24 -c <ip> -i <id> -k <key> -e GCM`
- Set mode: `python3 gree.py set Mod=1 -c <ip> -i <id> -k <key> -e GCM`
- Combined: `python3 gree.py set Pow=1 Mod=1 SetTem=24 -c <ip> -i <id> -k <key> -e GCM`

## Mode Values
- `Mod=0` = Auto
- `Mod=1` = Cool
- `Mod=2` = Dry
- `Mod=3` = Fan
- `Mod=4` = Heat

## Known Pitfalls
- Do not use ECB encryption — both ACs use GCM. Always pass `-e GCM`.
- The device may not respond to broadcast scan; target the IP directly.
- Bind is required only once; the key is stable and stored in memory.
- A fresh UDP socket is needed for each bind attempt if the first times out.
- TemSen returns a raw sensor value, not degrees Celsius directly.

## Fallbacks
- If the script fails with a timeout, the AC may be offline or the IP may have changed. Ping first.
- If the key was lost or bind fails, re-scan and re-bind the device using the IP directly.
- For a full scan, run: `python3 gree.py search -b 192.168.0.255` (may find 0 devices; direct IP works better).

## Validation
- After a `set` command, run `get` to verify the parameters changed.
- Check exit code and look for `r":200` in the response to confirm success.

## Stop Conditions
- If the AC does not respond to ping, report it offline rather than retrying.
- If bind fails repeatedly with different encryption types, ask the user for device model/firmware version.

[DATA]
ac.birou_etaj.ip=192.168.0.112
ac.birou_etaj.id=580d0d2e3bc8
ac.birou_etaj.key=mmvII30ecJ9i4Qwu
ac.birou_etaj.encryption=GCM
ac.birou_etaj.ver=V3.4.M
ac.birou_etaj.name=Birou Etaj
ac.birou_etaj.port=7000
ac.birou_etaj.power_plug=Priza_mon_08
ac.dormitor_albert.ip=192.168.0.100
ac.dormitor_albert.id=580d0d2e3b1e
ac.dormitor_albert.key=dz7ER725GNOt2W9b
ac.dormitor_albert.encryption=GCM
ac.dormitor_albert.ver=V3.4.M
ac.dormitor_albert.name=Dorm Albert
ac.dormitor_albert.port=7000
ac.dormitor_albert.power_plug=HeatingDormAlbert
ac.generic_key=a3K8Bx%2r8Y7#xDh
ac.generic_key_gcm={yxAHAY_Lm6pbC/<
