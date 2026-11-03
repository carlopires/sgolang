module example.com/net-shared-listener-accept

sgo 0.12.0
grant listen unix:shared.sock
grant connect unix:shared.sock
