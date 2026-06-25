[DESCRIPTION]
Network topology, server IPs, roles, and SSH info.

[DATA]
network.subnet=192.168.0.0/22
network.gateway=192.168.0.1
network.dns=192.168.0.1
network.ssh_servers=192.168.0.104,192.168.0.110,192.168.0.111
server.main.ip=192.168.0.104
server.main.hostname=nas
server.main.user=nas
server.main.pass=gheorghe
server.main.os=Debian 11 (kernel 6.10.11)
server.main.ssh=OpenSSH_8.4p1
server.main.role=Web server (OpenResty), Docker host (Portainer CE)
server.main.ports=22,80,443,53,9000
server.node.ip=192.168.0.110
server.node.hostname=node
server.node.user=node
server.node.pass=gheorghe
server.node.os=Debian 12 (kernel 6.1.0-41)
server.node.ssh=OpenSSH_9.2p1
server.node.role=Smart home server (Node-RED, Zigbee2MQTT, MQTT broker)
server.node.ports=22,8080,1880,1883
server.backup.ip=192.168.0.111
server.backup.hostname=upsmonitor
server.backup.user=ups
server.backup.pass=ups
server.backup.os=Debian 12 (kernel 6.1.0-37)
server.backup.ssh=OpenSSH_9.2p1
server.backup.role=Docker host (Portainer CE)
server.backup.ports=22,9000
