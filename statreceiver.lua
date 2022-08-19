-- possible sources:
--  * udpin(address)
--  * filein(path)
-- multiple sources can be handled in the same run (including multiple sources
-- of the same type) by calling deliver more than once.
source = udpin(":9000")

-- These two numbers are the size of destination metric buffers and packet buffers
-- respectively. Wrapping a metric destination in a metric buffer starts a goroutine
-- for that destination, which allows for concurrent writes to destinations, instead
-- of writing to destinations synchronously. If one destination blocks, this allows
-- the other destinations to continue, with the caveat that buffers may get overrun
-- if the buffer fills past this value.
-- One additional caveat to make sure mbuf and pbuf work - they need mbufprep and
-- pbufprep called higher in the pipeline. By default to save CPU cycles, memory
-- is reused, but this is bad in buffered writer situations. mbufprep and pbufprep
-- stress the garbage collector and lower performance at the expense of getting
-- mbuf and pbuf to work.
-- I've gone through and added mbuf and pbuf calls in various places. I think one
-- of our output destinations was getting behind and getting misconfigured, and
-- perhaps that was causing the holes in the data.
-- - JT 2019-05-15
mbufsize = 10000
pbufsize = 1000

-- multiple metric destination types
--  * graphite(address) goes to tcp with the graphite wire protocol
--  * print() goes to stdout
--  * db("sqlite3", path) goes to sqlite
--  * db("postgres", connstring) goes to postgres

influx_base = "http://influx-internal.datasci.storj.io:8086"
influx_user = os.getenv("INFLUX_USERNAME")
influx_pass = os.getenv("INFLUX_PASSWORD")

influx_wp_base = "http://influx-wp.datasci.storj.io:8086"
influx_wp_user = os.getenv("INFLUX_WP_USERNAME")
influx_wp_pass = os.getenv("INFLUX_WP_PASSWORD")

influx2_base = "http://influx2.datasci.storj.io:8086"
influx2_token = os.getenv("INFLUX_2_TOKEN")

v3_url = string.format("%s/write?db=v3_stats_new&u=%s&p=%s",influx_base, influx_user, influx_pass)
wp_url = string.format("%s/write?db=v3_stats_new&u=%s&p=%s",influx_wp_base, influx_wp_user, influx_wp_pass)
inf2_v3_url = string.format("%s/api/v2/write?bucket=v3_stats_new/autogen&org=storj&authorization=%s",influx2_base,influx2_token)

influx_out_v3 = influx(v3_url)
influx_out_wp = influx(wp_url)
influx2_out_v3 = influx(inf2_v3_url)

--    mbuf(graphite_out_stefan, mbufsize),
  -- send specific storagenode data to the db
    --keyfilter(
      --"env\\.process\\." ..
        --"|hw\\.disk\\..*Used" ..
        --"|hw\\.disk\\..*Avail" ..
        --"|hw\\.network\\.stats\\..*\\.(tx|rx)_bytes\\.(deriv|val)",
      --mbuf(db_out, mbufsize))



v3_metric_handlers = mbufprep(mbuf("influx_new", influx_out_v3, mbufsize))
wp_metric_handlers = mbufprep(mbuf("influx_new", influx_out_wp, mbufsize))
inf2_v3_metric_handlers = mbufprep(mbuf("influx_new", influx2_out_v3, mbufsize))

allowed_instance_id_applications = "(orbiter|healthcheck|satellite|retrievability|webproxy|gateway-mt|linksharing|authservice)"

-- create a metric parser.
metric_parser = parse(zeroinstanceifnot(allowed_instance_id_applications, v3_metric_handlers))
inf2_metric_parser = parse(zeroinstanceifnot(allowed_instance_id_applications, inf2_v3_metric_handlers))
wp_metric_parser = parse(wp_metric_handlers)

    --packetfilter(".*", "", udpout("localhost:9002")))
    --packetfilter("(storagenode|satellite)-(dev|prod|alphastorj|stagingstorj)", ""))

af = "(orbiter|linksharing|gateway-mt|authservice|satellite|retrievability-checker|downloadData|uploadData|healthcheck).*(-alpha|-release|storj|-transfersh)"
af_rothko = "(linksharing|gateway-mt|authservice|satellite|retrievability-checker|storagenode|uplink).*(-alpha|-release|storj|-transfersh)"
af_webproxy = "(webproxy|healthcheck).*(-alpha|-release|storj|-transfersh)"

uplink_header_matcher = headermultivalmatcher("sat",
    "12EayRS2V1kEsWESU9QMRseFhdxYxKicsiFmxrsLZHeLUtdps3S@us-central-1.tardigrade.io:7777",
    "12EayRS2V1kEsWESU9QMRseFhdxYxKicsiFmxrsLZHeLUtdps3S@mars.tardigrade.io:7777",
    "121RTSDpyNZVcEU84Ticf2L1ntiuUimbWgfATz21tuvgk3vzoA6@asia-east-1.tardigrade.io:7777",
    "121RTSDpyNZVcEU84Ticf2L1ntiuUimbWgfATz21tuvgk3vzoA6@saturn.tardigrade.io:7777",
    "12L9ZFwhzVpuEKMUNUqkaTLGzwY9G24tbiigLiXpmZWKwmcNDDs@europe-west-1.tardigrade.io:7777",
    "12L9ZFwhzVpuEKMUNUqkaTLGzwY9G24tbiigLiXpmZWKwmcNDDs@jupiter.tardigrade.io:7777",
    "118UWpMCHzs6CvSgWd9BfFVjw5K9pZbJjkfZJexMtSkmKxvvAW@satellite.stefan-benten.de:7777",
    "1wFTAgs9DP5RSnCqKV1eLf6N9wtk4EAtmN5DpSxcs8EjT69tGE@saltlake.tardigrade.io:7777")

-- pcopy forks data to multiple outputs
-- output types include parse, fileout, and udpout
destination = pbufprep(pcopy(
  --fileout("dump.out"),
  pbuf(packetfilter(af, "", nil, metric_parser), pbufsize),

  -- influx2
  pbuf(packetfilter(af, "", nil, inf2_metric_parser), pbufsize),

  -- webproxy dedicated
  pbuf(packetfilter(af_webproxy, "", nil, wp_metric_parser), pbufsize),

  -- useful local debugging
  pbuf(udpout("localhost:9001"), pbufsize),

  -- rothko
   pbuf(packetfilter(af_rothko, "", nil, udpout("rothko-internal.datasci.storj.io:9002")), pbufsize)

   -- uplink
   --pbuf(packetfilter("uplink", "", uplink_header_matcher, packetprint()), pbufsize)
 ))

-- tie the source to the destination
deliver(source, destination)
