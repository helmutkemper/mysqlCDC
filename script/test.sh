mysql -uroot -ppass -e "CREATE USER 'admin'@'%' IDENTIFIED BY 'admin';"
mysql -uroot -ppass -e "GRANT ALL PRIVILEGES ON *.* TO 'admin'@'%' WITH GRANT OPTION;"
mysql -uroot -ppass -e "FLUSH PRIVILEGES;"
