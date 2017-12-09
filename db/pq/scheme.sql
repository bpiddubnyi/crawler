CREATE TABLE uptime_log (
    time     TIMESTAMP NOT NULL,
    url      TEXT NOT NULL,
    local_ip TEXT NOT NULL,
    up       BOOLEAN DEFAULT false NOT NULL,
    UNIQUE(time, url)
);