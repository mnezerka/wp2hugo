package wordpress

import (
	"encoding/xml"
	"fmt"
)

type Rss struct {
	XMLName  xml.Name  `xml:"rss"`
	Channels []Channel `xml:"channel"`
}

type Channel struct {
	XMLName     xml.Name `xml:"channel"`
	Title       string   `xml:"title"`
	Description string   `xml:"description"`
	Items       []Item   `xml:"item"`
}

type Item struct {
	XMLName       xml.Name       `xml:"item"`
	Id            int            `xml:"http://wordpress.org/export/1.2/ post_id"`
	Name          string         `xml:"http://wordpress.org/export/1.2/ post_name"`
	ParentId      int            `xml:"http://wordpress.org/export/1.2/ post_parent"`
	Title         string         `xml:"title"`
	Creator       string         `xml:"http://purl.org/dc/elements/1.1/ creator"`
	Content       string         `xml:"http://purl.org/rss/1.0/modules/content/ encoded"`
	Type          string         `xml:"http://wordpress.org/export/1.2/ post_type"`
	MenuOrder     int            `xml:"http://wordpress.org/export/1.2/ menu_order"`
	PostDate      wp_date        `xml:"http://wordpress.org/export/1.2/ post_date"`
	Categories    []ItemCategory `xml:"category"`
	AttachmentUrl string         `xml:"http://wordpress.org/export/1.2/ attachment_url"`

	Attachments []Item
}

func (item *Item) GetTaxonomies() (map[string][]string, error) {

	result := map[string][]string{}

	//		"tags": [],
	//	"categories": [],
	//}

	for i := 0; i < len(item.Categories); i++ {
		c := item.Categories[i]
		switch c.Domain {
		case "post_tag":
			result["tags"] = append(result["tags"], c.Title)
		case "category":
			result["categories"] = append(result["categories"], c.Title)
		default:
			return result, fmt.Errorf("Unknown taxonomy (domain): %s", c.Domain)
		}
	}

	return result, nil
}

type ItemCategory struct {
	XMLName xml.Name `xml:"category"`
	Domain  string   `xml:"domain,attr"`
	Name    string   `xml:"nicename,attr"`
	Title   string   `xml:",chardata"`
}

type HugoFrontMatter struct {
	Title      string                    `yaml:"title"`
	Date       string                    `yaml:"date"`
	Slug       string                    `yaml:"slug"`
	Categories []string                  `yaml:"categories"`
	Tags       []string                  `yaml:"tags"`
	Resources  []HugoFrontMatterResource `yaml:"resources"`
}

type HugoFrontMatterResource struct {
	Src    string                 `yaml:"src"`
	Title  string                 `yaml:"title"`
	Params map[string]interface{} `yaml:"params"`
}
