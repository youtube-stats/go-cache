package main

import (
	"./message"
	"database/sql"
	"encoding/binary"
	"fmt"
	"github.com/golang/protobuf/proto"
	"math/rand"
	"net"
	"os"
	"time"

	_ "github.com/lib/pq"
)

type ChannelRow struct {
	id int32
	serial string
}

const (
	port = "0.0.0.0:3334"
	sleepTime = 3600
	sqlUrl = "postgresql://admin@localhost:5432/youtube?sslmode=disable"
	sqlQuery = "SELECT id, serial FROM youtube.stats.channels ORDER BY id ASC"
)

var (
	rows []ChannelRow
)

func getRandomRows(limit int) []byte {
	var top int
	if limit > len(rows) {
		top = len(rows)
	}

	n := 50
	// record indexes here to prevent duplicates
	indexes := make(map[int]bool)

	// create n random indexes
	for i := 0; i < n; i++ {
		var r int
		for {
			r = rand.Intn(top)
			if indexes[r] {
				continue
			}
			break
		}

		indexes[r] = true
	}

	var protoMsg message.ChannelMessage
	for i := range indexes {
		protoMsg.Ids = append(protoMsg.Ids, rows[i].id)
		protoMsg.Serials = append(protoMsg.Serials, rows[i].serial)
	}

	fmt.Println("Sending", protoMsg)

	data, err := proto.Marshal(&protoMsg)
	if err != nil {
		fmt.Println(err)
	}

	return data
}

func handleConnection(c net.Conn) {
	defer func() {
		err := c.Close()
		if err != nil {
			fmt.Println(err)
			os.Exit(4)
		}
	}()

	bytes := make([]byte, 4)
	n, err := c.Read(bytes)
	if err != nil {
		fmt.Println(err)
		os.Exit(7)
	}

	if n != 4 {
		fmt.Println("Need to read 4 bytes - received", n)
		return
	}

	fmt.Println("Retrieved", n, "bytes")
	limit := int(binary.LittleEndian.Uint32(bytes))
	fmt.Println("Limit is", limit)

	protoBytes := getRandomRows(limit)
	{
		_, err := c.Write(protoBytes)
		{
			if err != nil {
				fmt.Println(err)
				os.Exit(4)
			}
		}
	}
}

func setChannels() {
	fmt.Println("Updating channels")

	db, err := sql.Open("postgres", sqlUrl)
	if err != nil {
		fmt.Println(err)
		os.Exit(5)
	}

	defer func() {
		fmt.Println("Closing sql connection")
		_ = db.Close()
	}()

	results, err := db.Query(sqlQuery)
	if err != nil {
		fmt.Println(err)
		os.Exit(6)
	}

	tmp := make([]ChannelRow, 0)
	var row ChannelRow

	for results.Next() {
		err := results.Scan(&row.id, &row.serial)
		if err != nil {
			fmt.Println(err)
			os.Exit(7)
		}
		tmp = append(tmp, row)
	}

	fmt.Println("Retrieved", len(tmp), "channels")
	rows = tmp
}

func channelUpdate() {
	for {
		fmt.Println("Waiting for", sleepTime, "seconds until repulling channels")
		time.Sleep(sleepTime * time.Second)
		setChannels()
	}
}

func init() {
	fmt.Println("Cache service started")
	rand.Seed(time.Now().Unix())

	setChannels()
}

func main() {
	server, err := net.Listen("tcp4", port)
	{
		if err != nil {
			fmt.Println(err)
			os.Exit(2)
		}
	}

	defer func() {
		_ = server.Close()
	}()

	go channelUpdate()

	for {
		fmt.Println("Waiting for connection")
		connection, err := server.Accept()
		if err != nil {
			fmt.Println(err)
			fmt.Println("Closing server")
			_ = server.Close()
			os.Exit(3)
		}
		go handleConnection(connection)
	}
}
