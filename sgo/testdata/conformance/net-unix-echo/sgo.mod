module example.com/net-unix-echo

sgo 0.10.1
grant listen unix:echo.sock
grant connect unix:echo.sock
