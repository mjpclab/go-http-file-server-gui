go install github.com/fyne-io/fyne-cross@latest
fyne_cross_bin="$(go env GOPATH)/bin/fyne-cross"

cd "$(dirname $0)/../"
"$fyne_cross_bin" linux -arch=amd64 -release -env GOPROXY='https://goproxy.cn,direct'
