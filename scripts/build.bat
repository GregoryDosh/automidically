@ECHO OFF

@ECHO Building automidically...

@SET "AM_ROOT=%~dp0.."

del "%AM_ROOT%\automidically.exe*"
@REM go get github.com/akavel/rsrc
rsrc -arch amd64 -manifest assets\build.manifest  -ico assets\main.ico -o cmd\automidically\rsrc.syso
go mod tidy
go build -ldflags "-H=windowsgui -s -w -X main.defaultLogFilename=automidically.log" -o "%AM_ROOT%\automidically.exe" "%AM_ROOT%\cmd\automidically"
