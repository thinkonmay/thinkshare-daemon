mkdir msi
$go = New-Object net.webclient
$go.Downloadfile("https://go.dev/dl/go1.20.3.windows-amd64.msi"                                                                                                                      ,"msi/go.msi")
$git = New-Object net.webclient
$git.Downloadfile("https://github.com/git-for-windows/git/releases/download/v2.40.0.windows.1/Git-2.40.0-64-bit.exe"                                                                 ,"msi/git.exe")
$cruntime = New-Object net.webclient
$cruntime.Downloadfile("https://aka.ms/vs/17/release/vc_redist.x64.exe"                                                                                                                    ,"msi/cruntime.exe")

./msi/git.exe /SILENT
./msi/cruntime.exe /passive
Start-Process ./msi/go.msi  -ArgumentList "/qb" -Wait


git clone https://github.com/thinkonmay/thinkshare-daemon daemon
cd daemon
git submodule update --init
