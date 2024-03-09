if not exist "build" mkdir "build"
go build -o build\TunicTransitionTracker-windows.exe
SET GOOS=linux
go build -o build\TunicTransitionTracker-linux
