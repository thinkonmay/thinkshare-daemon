$env:Path += ';C:\gstreamer\1.22.0\msvc_x86_64\bin'
$env:PKG_CONFIG_PATH = "C:\gstreamer\1.22.0\msvc_x86_64\lib\pkgconfig"

git clone "https://github.com/thinkonmay/thinkshare-daemon" daemon
git checkout master
Set-Location .\daemon

git submodule update --init --recursive

# build GO 
go build  -o daemon.exe ./cmd/

Set-Location .\hub
go build -o hub.exe  ./cmd/server/
Set-Location ../

# build .NET
Set-Location .\hid
dotnet build . --output "bin" --self-contained true --runtime win-x64
Set-Location ..


Compress-Archive . -DestinationPath .\thinkremote.zip 