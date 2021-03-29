package main

import (
	"database/sql"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/mount"
	"github.com/docker/go-connections/nat"
	_ "github.com/go-sql-driver/mysql"
	iotmakerdocker "github.com/helmutkemper/iotmaker.docker/v1.0.1"
	"io/fs"
	"io/ioutil"
	"log"
	"sync"

	binlog "github.com/granicus/mysql-binlog-go"
	"time"
)

var db *sql.DB

func main() {

	//var b []byte
	//f, e := os.Open("./log/mysql/mysql-bin.000003")
	//if e != nil {
	//	panic(e)
	//}
	//b, e = ioutil.ReadAll(f)
	//if e != nil {
	//	panic(e)
	//}
	//
	//fmt.Printf("%s", hex.Dump(b))
	//os.Exit(5)

	var imageName = "mysql/mysql-server:latest"
	//imageName = "mysql/mysql-server:5.6"

	var err error
	var pullStatusChannel = iotmakerdocker.NewImagePullStatusChannel()
	var newPort nat.Port
	var mountList []mount.Mount
	var defaultMySQLPort nat.Port
	var containerId string

	var dockerSys = iotmakerdocker.DockerSystem{}
	err = dockerSys.Init()
	if err != nil {
		return
	}

	go func(c chan iotmakerdocker.ContainerPullStatusSendToChannel) {
		for {
			select {
			case status := <-c:
				log.Printf("image pull status: %+v\n", status)

				if status.Closed == true {
					log.Println("image pull complete!")
				}
			}
		}

	}(*pullStatusChannel)

	// stop and remove containers and garbage collector
	err = RemoveAllByNameContains("delete")
	if err != nil {
		panic(err)
	}

	defaultMySQLPort, err = nat.NewPort("tcp", "3306")
	if err != nil {
		return
	}

	newPort, err = nat.NewPort("tcp", "3306")
	if err != nil {
		return
	}

	portMap := nat.PortMap{
		// container port number/protocol [tpc/udp]
		defaultMySQLPort: []nat.PortBinding{ // server original port
			{
				// server output port number
				HostPort: newPort.Port(),
			},
		},
	}

	var config = container.Config{
		OpenStdin:    true,
		AttachStderr: true,
		AttachStdin:  true,
		AttachStdout: true,
		Env: []string{
			"MYSQL_ROOT_PASSWORD=pass",
			"MYSQL_DATABASE=test",
			"bind-address=0.0.0.0",
			"defaults-file=/etc/my.cnf",
			"initialize-insecure",
			"datadir=/var/lib/mysql",
			"console",
			"MYSQL_ROOT_HOST=0.0.0.0",
			"expire_logs_days=10",
			"max_binlog_size=100M",
			"binlog-format=row",
			"log_bin=/var/log/mysql/mysql-bin.log",
		},
		Image: imageName,
	}

	ml := []iotmakerdocker.Mount{
		{
			MountType:   iotmakerdocker.KVolumeMountTypeBind,
			Source:      "./log",
			Destination: "/var/log",
		},
		{
			MountType:   iotmakerdocker.KVolumeMountTypeBind,
			Source:      "./script/my:latest.cnf",
			Destination: "/etc/my.cnf",
		},
	}

	// define an external MySQL config file path
	mountList, err = iotmakerdocker.NewVolumeMount(ml)
	if err != nil {
		return
	}

	_, _, err = dockerSys.ImagePull(config.Image, pullStatusChannel)
	if err != nil {
		return
	}

	containerId, err = dockerSys.ContainerCreateWithConfig(
		&config,
		"container_delete_before_test",
		iotmakerdocker.KRestartPolicyNo,
		portMap,
		mountList,
		nil,
	)
	if err != nil {
		return
	}

	err = dockerSys.ContainerStart(containerId)
	if err != nil {
		panic(err)
	}

	switch imageName {
	case "mysql/mysql-server:latest":
		_, err = dockerSys.ContainerLogsWaitText(containerId, "/usr/sbin/mysqld: ready for connection", log.Writer())
		if err != nil {
			panic(err)
		}

	case "mysql/mysql-server:5.6":
		_, err = dockerSys.ContainerLogsWaitText(containerId, "/usr/sbin/mysqld (mysqld 5.6.51) starting as process", log.Writer())
		if err != nil {
			panic(err)
		}

	}

	time.Sleep(30 * time.Second)

	var exitCode int
	var runing bool
	exitCode, runing, err = dockerSys.ContainerExecCommand(containerId, []string{`mysql`, `-uroot`, `-ppass`, `-e`, `CREATE USER 'admin'@'%' IDENTIFIED BY 'admin';`})
	if err != nil {
		panic(err)
	}
	log.Printf("exitCode: %v, runing: %v\n", exitCode, runing)

	exitCode, runing, err = dockerSys.ContainerExecCommand(containerId, []string{`mysql`, `-uroot`, `-ppass`, `-e`, `GRANT ALL PRIVILEGES ON *.* TO 'admin'@'%' WITH GRANT OPTION;`})
	if err != nil {
		panic(err)
	}
	log.Printf("exitCode: %v, runing: %v\n", exitCode, runing)

	exitCode, runing, err = dockerSys.ContainerExecCommand(containerId, []string{`mysql`, `-uroot`, `-ppass`, `-e`, `FLUSH PRIVILEGES;`})
	if err != nil {
		panic(err)
	}
	log.Printf("exitCode: %v, runing: %v\n", exitCode, runing)

	var loopLimit = 60
	var wg sync.WaitGroup
	wg.Add(1)

	go func() {
		var err error
		for {
			time.Sleep(time.Second)
			loopLimit -= 1
			if loopLimit < 0 {
				panic(err)
			}

			db, err = sql.Open("mysql", "admin:admin@/test")
			if err != nil {
				continue
			}

			db.SetConnMaxLifetime(time.Minute * 3)
			db.SetMaxOpenConns(10)
			db.SetMaxIdleConns(10)

			err = db.Ping()
			if err != nil {
				continue
			}

			wg.Done()
			return
		}
	}()
	wg.Wait()

	//err = CreateDatabase()
	//if err != nil {
	//	panic(err)
	//}

	err = CreateTable()
	if err != nil {
		panic(err)
	}

	for i := 0; i != 3; i += 1 {
		err = Set(
			"5996b891-9d3c-4038-af37-cb07f5f0f72d",
			1,
			"Fulano da Silva Sauro",
			"Sauro",
			"sauro@pangea.com",
			"admin",
		)
		if err != nil {
			panic(err)
		}
	}

	err = Update()

	if err != nil {
		panic(err)
	}

	time.Sleep(time.Second * 5)

	var dir []fs.FileInfo
	var logMysql *binlog.Binlog
	dir, err = ioutil.ReadDir("./log/mysql")
	if err != nil {
		panic(err)
	}

	for _, dirData := range dir {
		if dirData.IsDir() == true {
			continue
		}

		logMysql, err = binlog.OpenBinlog("./log/mysql/" + dirData.Name())
		if err != nil {
			panic(err)
		}

		for _, event := range logMysql.Events() {
			if event.Type() == binlog.WRITE_ROWS_EVENTv2 {
				rowsEvent := event.Data().(*binlog.RowsEvent)

				log.Println("Found some rows that were inserted:", rowsEvent.Rows)
			}
		}
	}

	//err = RemoveAllByNameContains("delete")
	if err != nil {
		panic(err)
	}

}

func Update() (err error) {
	var statement *sql.Stmt
	statement, err = db.Prepare(
		`UPDATE user SET
    name = ?
WHERE
    id = ?;`,
	)
	if err != nil {
		log.Printf("SQLiteUser.Set().error: %v", err.Error())
		return
	}

	_, err = statement.Exec("name changed", 1)
	if err != nil {
		log.Printf("SQLiteUser.Set().error: %v", err.Error())
	}
	return
}

func Set(idMenu string, admin int, name, nickName, eMail, password string) (err error) {
	var statement *sql.Stmt
	statement, err = db.Prepare(
		`INSERT INTO user (menuId, admin, name, nickName, eMail, password) VALUES(?, ?, ?, ?, ?, ?)`,
	)
	if err != nil {
		log.Printf("SQLiteUser.Set().error: %v", err.Error())
		return
	}

	_, err = statement.Exec(idMenu, admin, name, nickName, eMail, password)
	if err != nil {
		log.Printf("SQLiteUser.Set().error: %v", err.Error())
	}
	return
}

func CreateTable() (err error) {
	var statement *sql.Stmt
	statement, err = db.Prepare(`
		CREATE TABLE IF NOT EXISTS
    	user (
				id INT(6) UNSIGNED AUTO_INCREMENT PRIMARY KEY,
				menuId VARCHAR(255),        -- id menu list
				admin INTEGER,         -- 0: normal user; 1 admin user
				name VARCHAR(255),						 -- complete name
				nickName VARCHAR(255),				 -- nick name
				eMail VARCHAR(255),						 -- e-mail
				password VARCHAR(255)				 -- password
			);
		`,
	)
	if err != nil {
		log.Printf("SQLiteUser.createTableUser().error: %v", err.Error())
		return
	}

	_, err = statement.Exec()
	if err != nil {
		log.Printf("SQLiteUser.createTableUser().error: %v", err.Error())
	}

	return
}

func CreateDatabase() (err error) {
	var statement *sql.Stmt
	statement, err = db.Prepare(`
		CREATE DATABASE test;
		`,
	)
	if err != nil {
		log.Printf("SQLiteUser.CreateDatabase().error: %v", err.Error())
		return
	}

	_, err = statement.Exec()
	if err != nil {
		log.Printf("SQLiteUser.CreateDatabase().error: %v", err.Error())
	}

	return
}

func RemoveAllByNameContains(name string) (err error) {
	var dockerSys = iotmakerdocker.DockerSystem{}
	err = dockerSys.Init()
	if err != nil {
		return
	}

	err = dockerSys.RemoveAllByNameContains(name)

	return
}
