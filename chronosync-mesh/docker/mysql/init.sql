CREATE USER IF NOT EXISTS 'chronosync'@'%' IDENTIFIED BY 'chronosync_password';
GRANT ALL PRIVILEGES ON chronosync_db.* TO 'chronosync'@'%';
FLUSH PRIVILEGES;

CREATE TABLE IF NOT EXISTS user_profiles (
  id BIGINT NOT NULL PRIMARY KEY,
  username VARCHAR(255) NOT NULL,
  email VARCHAR(320) NOT NULL
);
