package wordpress

import (
	"encoding/xml"
	"time"
)

type wp_date struct {
	time.Time
}

/* Wordpress XML specific format of encoding dates

This is implementation of custom XML decoder, which is called automaticaaly
by encoding/xml methods (e.g. Unmarshall) */
func (c *wp_date) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	const wp_format = "2006-01-02 15:04:05" // yyyy-mm-dd hh:mm:dd date format
	var v string
	d.DecodeElement(&v, &start)
	parse, err := time.Parse(wp_format, v)
	if err != nil {
		return err
	}
	*c = wp_date{parse}
	return nil
}
