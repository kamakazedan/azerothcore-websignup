-- The MySQL user the sign-up site logs in as, with only the rights it
-- needs: look up and add accounts, set a new password, read the realm's
-- name and uptime, and read characters for the online and top lists. It
-- can't delete anything or read any other table.
--
-- setup.sh runs this for you. To run it by hand, without writing the
-- password into this file, fill it in on the way through:
--
--   pw=$(grep '^DB_PASSWORD=' .env | cut -d= -f2)
--   sed "s/{{PASSWORD}}/'$pw'/g" sql/create-user.sql | docker exec -i ac-database mysql -uroot -p
--
-- If your databases aren't called acore_auth and acore_characters, change
-- those names too. Running it again is safe: it resets the password.

CREATE USER IF NOT EXISTS 'websignup'@'%' IDENTIFIED BY {{PASSWORD}};
ALTER USER 'websignup'@'%' IDENTIFIED BY {{PASSWORD}};

GRANT SELECT (`id`, `username`, `salt`, `verifier`, `email`, `reg_mail`),
      INSERT (`username`, `salt`, `verifier`, `expansion`, `reg_mail`, `email`, `joindate`),
      UPDATE (`salt`, `verifier`)
   ON `acore_auth`.`account` TO 'websignup'@'%';
GRANT SELECT (`id`, `name`, `icon`) ON `acore_auth`.`realmlist` TO 'websignup'@'%';
GRANT SELECT (`realmid`, `starttime`) ON `acore_auth`.`uptime` TO 'websignup'@'%';
GRANT INSERT (`realmid`, `acctid`, `numchars`) ON `acore_auth`.`realmcharacters` TO 'websignup'@'%';
GRANT SELECT (`account`, `name`, `race`, `class`, `level`, `totaltime`, `online`)
   ON `acore_characters`.`characters` TO 'websignup'@'%';
