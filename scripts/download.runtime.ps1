mkdir msi
$dotnet = New-Object net.webclient
$dotnet.Downloadfile("https://download.visualstudio.microsoft.com/download/pr/c6ad374b-9b66-49ed-a140-588348d0c29a/78084d635f2a4011ccd65dc7fd9e83ce/dotnet-sdk-7.0.202-win-x64.exe"  ,"msi/dotnet.exe")
$go = New-Object net.webclient
$go.Downloadfile("https://go.dev/dl/go1.20.3.windows-amd64.msi"                                                                                                                      ,"msi/go.msi")
$git = New-Object net.webclient
$git.Downloadfile("https://github.com/git-for-windows/git/releases/download/v2.40.0.windows.1/Git-2.40.0-64-bit.exe"                                                                 ,"msi/git.exe")
$cruntime = New-Object net.webclient
$cruntime.Downloadfile("https://aka.ms/vs/17/release/vc_redist.x64.exe"                                                                                                                    ,"msi/cruntime.exe")

./msi/git.exe /SILENT
./msi/cruntime.exe /passive
Start-Process ./msi/go.msi  -ArgumentList "/qb" -Wait                      
./msi/dotnet.exe  /passive       


