module example.com/lab-net-unix-echo

sgo 0.16.1
grant listen unix:echo.sock
grant connect unix:echo.sock
