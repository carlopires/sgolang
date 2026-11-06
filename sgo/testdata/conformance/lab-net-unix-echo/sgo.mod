module example.com/lab-net-unix-echo

sgo 0.22.0
grant listen unix:echo.sock
grant connect unix:echo.sock
