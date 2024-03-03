apt-get update -y
apt-get install -y snapd git net-tools virt-manager openvswitch-switch driverctl neofetch vim

echo "[Unit]
Description=
After=network.target

StartLimitIntervalSec=500
StartLimitBurst=5

[Service]
Type=simple
ExecStart=/home/huyhoang/daemon/daemon
WorkingDirectory=/home/huyhoang/daemon

Restart=always
RestartSec=5s

[Install]
WantedBy=multi-user.target" > /lib/systemd/system/virtdaemon.service

sudo vim /etc/libvirt/qemu.conf
# append user = "root"
sudo systemctl restart libvirtd
neofetch