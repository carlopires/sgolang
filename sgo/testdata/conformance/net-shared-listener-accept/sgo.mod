module example.com/net-shared-listener-accept

sgo 0.20.0
grant listen unix:shared.sock
grant connect unix:shared.sock
