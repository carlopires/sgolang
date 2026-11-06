module example.com/net-unix-echo

sgo 0.21.0
grant listen unix:echo.sock
grant connect unix:echo.sock
