module example.com/net-shared-listener-accept

sgo 0.20.1
grant listen unix:shared.sock
grant connect unix:shared.sock
