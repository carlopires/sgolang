module example.com/net-unix-echo

sgo 0.12.0
grant listen unix:echo.sock
grant connect unix:echo.sock
