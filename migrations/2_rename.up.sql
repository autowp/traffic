RENAME TABLE 
    ip_monitoring4 TO ip_monitoring,
    banned_ip TO ip_ban,
    referer TO hotlink_referer,
    referer_whitelist TO hotlink_whitelist,
    referer_blacklist TO hotlink_blacklist;

ALTER TABLE ip_ban RENAME COLUMN up_to TO until;