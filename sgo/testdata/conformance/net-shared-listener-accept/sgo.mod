module example.com/net-shared-listener-accept

sgo 0.14.0
grant listen unix:shared.sock
grant connect unix:shared.sock
