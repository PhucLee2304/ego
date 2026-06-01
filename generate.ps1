param (
    [string]$Command = ""
)

if ($Command -eq "") {
    Write-Host "Please specify a command: swag, protoc, or all"
    exit
}

if ($Command -eq "swag" -or $Command -eq "all") {
    Write-Host "Generating Swagger docs for all services..."
    $services = Get-ChildItem -Path "services" -Directory
    foreach ($svc in $services) {
        $mainGoPath = Join-Path $svc.FullName "cmd\main.go"
        if (Test-Path $mainGoPath) {
            Write-Host "--> Generating swag for $($svc.Name)"
            Push-Location $svc.FullName
            swag init -g cmd/main.go -o docs --parseDependency --parseInternal
            Pop-Location
        }
    }
}

if ($Command -eq "protoc" -or $Command -eq "all") {
    Write-Host "Generating proto files..."
    $protoDirs = Get-ChildItem -Path "api\proto" -Directory
    foreach ($dir in $protoDirs) {
        Write-Host "--> Generating proto for $($dir.Name)"
        $protoFiles = Get-ChildItem -Path $dir.FullName -Filter "*.proto"
        if ($protoFiles.Count -gt 0) {
            # Collect all .proto files as relative paths
            $relFiles = @()
            foreach ($f in $protoFiles) {
                $relFiles += "api/proto/$($dir.Name)/$($f.Name)"
            }
            
            # Use protoc.exe directly with relative paths
            protoc.exe -I . -I platform `
                --go_out=./api/gen/go --go_opt=module=ego/api/gen/go `
                --go-grpc_out=./api/gen/go --go-grpc_opt=module=ego/api/gen/go `
                --grpc-gateway_out=./api/gen/go --grpc-gateway_opt=module=ego/api/gen/go `
                $relFiles
        }
    }
}
