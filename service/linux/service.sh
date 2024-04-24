go build -o ./binary/daemon ./service/linux/
echo "[Unit]
Description=
After=network.target

StartLimitIntervalSec=500
StartLimitBurst=5

[Service]
Type=simple
User=huyhoang
ExecStart="$PWD"/binary/daemon
WorkingDirectory="$PWD"

Restart=always
RestartSec=5s

[Install]
WantedBy=multi-user.target" > /lib/systemd/system/virtdaemon.service
systemctl daemon-reload
systemctl restart virtdaemon
cpupower frequency-set -g performance		
journalctl -f -u virtdaemon