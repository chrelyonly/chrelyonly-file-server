set GOROOT=D:\dev\sdk\go\gopath\pkg\mod\golang.org\toolchain@v0.0.1-go1.26.4.windows-amd64
set GOPATH=D:\dev\dev\sdk\gopath
set CGO_ENABLED=0
set GOARCH=amd64
set GOOS=windows
D:\dev\sdk\go\gopath\pkg\mod\golang.org\toolchain@v0.0.1-go1.26.4.windows-amd64\bin\go.exe build -o D:\dev\project\chrelyonly-file-server\build\chrelyonly-file-server-amd64-windows main.go
set GOARCH=amd64
set GOOS=linux
D:\dev\sdk\go\gopath\pkg\mod\golang.org\toolchain@v0.0.1-go1.26.4.windows-amd64\bin\go.exe build -o D:\dev\project\chrelyonly-file-server\build\chrelyonly-file-server-amd64-darwin main.go
set GOARCH=amd64
set GOOS=darwin
D:\dev\sdk\go\gopath\pkg\mod\golang.org\toolchain@v0.0.1-go1.26.4.windows-amd64\bin\go.exe build -o D:\dev\project\chrelyonly-file-server\build\chrelyonly-file-server-amd64-linux main.go



go build -trimpath  -ldflags="-s -w" -o build/strawberry_minio main.go