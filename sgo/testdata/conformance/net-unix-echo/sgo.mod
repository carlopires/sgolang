module example.com/net-unix-echo

sgo 0.19.1
grant listen unix:echo.sock
grant connect unix:echo.sock
