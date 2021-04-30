package main

import (
	"database/sql"
	"errors"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/mount"
	"github.com/docker/go-connections/nat"
	_ "github.com/go-sql-driver/mysql"
	iotmakerdocker "github.com/helmutkemper/iotmaker.docker/v1.0.1"
	"io/fs"
	"io/ioutil"
	"log"
	"strings"
	"time"
)

var db *sql.DB

func main() {

	var imageName = "mysql/mysql-server:latest"
	imageName = "mysql/mysql-server:5.6"
	imageName = "mariadb:latest"

	var err error
	var pullStatusChannel = iotmakerdocker.NewImagePullStatusChannel()
	var newPort nat.Port
	var mountList []mount.Mount
	var defaultMySQLPort nat.Port
	var containerId string

	var exitCode int
	var runing bool
	var stdout, stderr []byte

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
			"MYSQL_ROOT_HOST=0.0.0.0",
			"expire_logs_days=10",
			"max_binlog_size=100M",
			"binlog-format=row",
			"log-bin=/var/log/mysql/mysql-bin.log",
			"log-basename=/var/log/mysql/mysql.log",
			"binlog_format=mixed",
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
			Destination: "/etc/mysql/my.cnf",
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

	//ready for connections. Version: '8.0.24'

	switch imageName {
	case "mariadb:latest":
		_, err = dockerSys.ContainerLogsWaitText(containerId, "socket: '/run/mysqld/mysqld.sock'  port: 3306", log.Writer())
		if err != nil {
			panic(err)
		}

	case "mysql/mysql-server:latest":
		_, err = dockerSys.ContainerLogsWaitText(containerId, "/usr/sbin/mysqld: ready for connections", log.Writer())
		if err != nil {
			panic(err)
		}

	case "mysql/mysql-server:5.6":
		_, err = dockerSys.ContainerLogsWaitText(containerId, "socket: '/var/lib/mysql/mysql.sock'  port: 3306  MySQL Community Server (GPL)", log.Writer())
		if err != nil {
			panic(err)
		}

	}

	exitCode, runing, stdout, stderr, err = dockerSys.ContainerExecCommand(containerId, []string{`mysql`, `-uroot`, `-ppass`, `-e`, `CREATE USER 'admin'@'%' IDENTIFIED BY 'admin';`})
	if err != nil {
		panic(err)
	}
	log.Printf("exitCode: %v, runing: %v\n", exitCode, runing)
	log.Printf("comando 1: %s\n", stdout)
	log.Printf("comando 1 err: %s\n", stderr)

	exitCode, runing, stdout, stderr, err = dockerSys.ContainerExecCommand(containerId, []string{`mysql`, `-uroot`, `-ppass`, `-e`, `GRANT ALL PRIVILEGES ON *.* TO 'admin'@'%' WITH GRANT OPTION;`})
	if err != nil {
		panic(err)
	}
	log.Printf("exitCode: %v, runing: %v\n", exitCode, runing)
	log.Printf("comando 1: %s\n", stdout)
	log.Printf("comando 1 err: %s\n", stderr)

	exitCode, runing, stdout, stderr, err = dockerSys.ContainerExecCommand(containerId, []string{`mysql`, `-uroot`, `-ppass`, `-e`, `FLUSH PRIVILEGES;`})
	if err != nil {
		panic(err)
	}
	log.Printf("exitCode: %v, runing: %v\n", exitCode, runing)
	log.Printf("comando 1: %s\n", stdout)
	log.Printf("comando 1 err: %s\n", stderr)

	var loopLimit = 60

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

		break
	}

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
			"Fulan'o da Silv\"a S`au(ro)",
			"Sauro",
			"sauro@pangea.com",
			"admin",
		)
		if err != nil {
			panic(err)
		}
	}

	err = Get()
	if err != nil {
		panic(err)
	}

	err = Update()
	if err != nil {
		panic(err)
	}

	time.Sleep(time.Second * 5)

	var dir []fs.FileInfo
	dir, err = ioutil.ReadDir("./log/mysql")
	if err != nil {
		panic(err)
	}

	for _, dirData := range dir {
		if dirData.IsDir() == true {
			continue
		}

		//***************************************************************************************************************************************************************************

		exitCode, runing, stdout, stderr, err = dockerSys.ContainerExecCommand(containerId, []string{`mysqlbinlog`, `-v`, `--base64-output=DECODE-ROWS`, `/var/log/mysql/` + dirData.Name()})
		if err != nil {
			panic(err)
		}

		sp := strings.Split(string(stdout), "/*!*/;")
		for _, v := range sp {

			if strings.Contains(v, "INSERT INTO time_zone") == true {
				continue
			}

			if strings.Contains(v, "end_log_pos") == true {
				continue
			}

			if strings.Contains(v, "COMMIT") == true {
				continue
			}

			if strings.Contains(v, "START TRANSACTION") == true {
				continue
			}

			v = strings.Trim(v, "\n")
			v = strings.Trim(v, "\r")
			log.Printf("%v", v)
		}

		//log.Printf("exitCode: %v, runing: %v\n", exitCode, runing)
		//log.Printf("comando 1: %s\n", stdout)
		//log.Printf("comando 1 err: %s\n", stderr)

		//***************************************************************************************************************************************************************************
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
				email VARCHAR(255),						 -- e-mail
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

func Get() (err error) {
	var rows *sql.Rows
	rows, err = db.Query(
		`
			SELECT
				id,
				admin,
				name,
				nickName,
				email,
				password
			FROM
				user
		`,
	)
	if err != nil {
		return
	}

	var id string
	var mail string
	var admin int
	var name string
	var nickName string
	var password string

	if rows.Next() {
		err = rows.Scan(&id, &admin, &name, &nickName, &mail, &password)
		if err != nil {
			return
		}

		log.Printf("id: %v, admin: %v, nickName: %v, email: %v, password: %v", id, admin, nickName, mail, password)

	} else {
		err = errors.New("getAll() error")
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
