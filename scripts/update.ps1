git submodule update --init --recursive

echo "building hub.exe with go"
Set-Location .\hub
go build -o hub.exe  ./cmd/server/
Set-Location ../

# build .NET
echo "building hid server with dotnet"
Set-Location .\hid
dotnet build . --output "bin" --self-contained true --runtime win-x64
Set-Location ..
