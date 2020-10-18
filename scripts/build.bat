@ECHO OFF

@ECHO Building automidically...

@SET "AM_ROOT=%~dp0.."

go get github.com/akavel/rsrc
rsrc -arch amd64 -manifest assets\build.manifest  -ico assets\main.ico -o cmd\automidically\rsrc.syso
go build -ldflags "-H=windowsgui -s -w" -o "%AM_ROOT%\am.exe" "%AM_ROOT%\cmd\automidically"
