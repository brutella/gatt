CC=gcc
DESTDIR=/usr/local/bin

all:
	$(CC) hci-ble.c -lbluetooth -o hci-ble
	$(CC) hci-ble-client.c -lbluetooth -o hci-ble-client
	$(CC) l2cap-ble.c -lbluetooth -o l2cap-ble
	$(CC) l2cap-ble-client.c -lbluetooth -o l2cap-ble-client

install:
	mv hci-ble $(DESTDIR)
	mv hci-ble-client $(DESTDIR)
	mv l2cap-ble $(DESTDIR)
	mv l2cap-ble-client $(DESTDIR)
	setcap 'CAP_NET_RAW=+ep' $(DESTDIR)/hci-ble
	setcap 'CAP_NET_RAW=+ep' $(DESTDIR)/hci-ble-client

uninstall:
	rm $(DESTDIR)/hci-ble
	rm $(DESTDIR)/hci-ble-client
	rm $(DESTDIR)/l2cap-ble
	rm $(DESTDIR)/l2cap-ble-client

clean:
	rm -f hci-ble hci-ble-client l2cap-ble l2cap-ble-client
