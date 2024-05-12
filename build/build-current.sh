go install fyne.io/fyne/v2/cmd/fyne@latest
fyne_bin="$(go env GOPATH)/bin/fyne"

cd "$(dirname $0)/../"
"$fyne_bin" package --release
