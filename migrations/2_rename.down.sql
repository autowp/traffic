RENAME TABLE 
    ip_monitoring TO ip_monitoring4,
    ip_ban TO banned_ip,
    hotlink_referer TO referer,
    hotlink_whitelist TO referer_whitelist,
    hotlink_blacklist TO referer_blacklist;

ALTER TABLE ip_ban RENAME COLUMN until TO up_to;