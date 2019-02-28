package tcpbinddev

import (
	"fmt"
	"net"
	"os"
	"syscall"

	"github.com/pkg/errors"
)

func TcpBindToDev(network, addr, device string, timeout int) (net.Conn, error) {
	if network == "" || addr == "" {
		return nil, errors.New("network or addr not set")
	}
	sa, soType, err := getSockaddr(network, addr)
	if err != nil {
		return nil, errors.Wrap(err, "getSockaddr->")
	}

	fd, err := newSocketCloexec(soType, syscall.SOCK_STREAM, syscall.IPPROTO_TCP)
	if err != nil {
		return nil, errors.Wrap(err, "newSocketCloexec->")
	}
	if device != "" {
		err = syscall.BindToDevice(fd, device)
		if err != nil {
			syscall.Close(fd)
			return nil, errors.Wrap(err, "BindToDevice->")
		}
	}
	fmt.Printf("fd:%d\n", fd)

	err = syscall.Connect(fd, sa)
	if err != nil && err.(syscall.Errno) != syscall.EINPROGRESS {
		//EINPROGRESS: The socket is nonblocking and the  connection  cannot  be  completed immediately.
		syscall.Close(fd)
		return nil, errors.Wrap(err, "Connect->")
	}
	err = connectTimeout(fd, timeout)
	if err != nil {
		syscall.Close(fd)
		return nil, errors.Wrap(err, "connectTimeout->")
	}

	//now set nonblocking
	// err = syscall.SetNonblock(fd, true)
	// if err != nil {
	// 	syscall.Close(fd)
	// 	return nil, errors.Wrap(err, "SetNonblock->")
	// }

	name := "tcp socket to netPoll"
	file := os.NewFile(uintptr(fd), name)
	conn, err := net.FileConn(file)
	if err != nil {
		file.Close()
		return nil, errors.Wrap(err, "FileConn->")
	}

	//close file does not affect conn
	if err := file.Close(); err != nil {
		conn.Close()
		return nil, errors.Wrap(err, "file.Close() error->")
	}
	return conn, nil
}

func getSockaddr(network, addr string) (sa syscall.Sockaddr, soType int, err error) {
	if network != "tcp4" && network != "tcp6" {
		return nil, -1, errors.New("only tcp4 and tcp6 network is supported")
	}

	tcpAddr, err := net.ResolveTCPAddr(network, addr)
	if err != nil {
		return nil, -1, err
	}

	switch network {
	case "tcp4":
		var sa4 syscall.SockaddrInet4
		sa4.Port = tcpAddr.Port
		copy(sa4.Addr[:], tcpAddr.IP.To4())
		return &sa4, syscall.AF_INET, nil
	case "tcp6":
		var sa6 syscall.SockaddrInet6
		sa6.Port = tcpAddr.Port
		copy(sa6.Addr[:], tcpAddr.IP.To16())
		if tcpAddr.Zone != "" {
			ifi, err := net.InterfaceByName(tcpAddr.Zone)
			if err != nil {
				return nil, -1, err
			}
			sa6.ZoneId = uint32(ifi.Index)
		}
		return &sa6, syscall.AF_INET6, nil
	default:
		return nil, -1, errors.New("Unknown network type " + network)
	}
}