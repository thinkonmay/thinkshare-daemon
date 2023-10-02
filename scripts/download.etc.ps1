$vnc = New-Object net.webclient
$vnc.Downloadfile("https://www.tightvnc.com/download/2.8.81/tightvnc-2.8.81-gpl-setup-64bit.msi", "msi/tightvnc.msi")
$vigem = New-Object net.webclient
$vigem.Downloadfile("https://github.com/nefarius/ViGEmBus/releases/download/v1.21.442.0/ViGEmBus_1.21.442_x64_x86_arm64.exe", "msi/vigem.exe")
$vigem = New-Object net.webclient
$vigem.Downloadfile("https://github.com/thinkonmay/thinkshare-daemon/releases/download/audio/VBCABLE_A_Driver_Pack43.zip", "msi/vbcablea.zip")
$vigem = New-Object net.webclient
$vigem.Downloadfile("https://github.com/thinkonmay/thinkshare-daemon/releases/download/audio/VBCABLE_Driver_Pack43.zip", "msi/vbcable.zip")