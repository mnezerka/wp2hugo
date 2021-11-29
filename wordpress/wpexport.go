package wordpress

import (
	"encoding/xml"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"

	md "github.com/JohannesKaufmann/html-to-markdown"
	"github.com/op/go-logging"
	"gopkg.in/yaml.v2"
)

type WpExport struct {
	log     *logging.Logger
	channel *Channel
}

func NewWpExport(logger *logging.Logger) *WpExport {
	wp_export := WpExport{}
	wp_export.log = logger

	wp_export.log.Debug("New instance of wordpress export created")

	return &wp_export
}

func (w *WpExport) check(err error) {
	if err != nil {
		w.log.Fatal(err)
	}
}

func (w *WpExport) ReadWpExport(file_path string) error {

	w.log.Infof("Reading export file from %s", file_path)

	// Open our xmlFile
	xmlFile, err := os.Open(file_path)
	// if we os.Open returns an error then handle it
	if err != nil {
		fmt.Println(err)
	}

	w.log.Infof("Successfully opened %s", file_path)

	// read our opened xmlFile as a byte array.
	byteValue, _ := ioutil.ReadAll(xmlFile)

	// we unmarshal our byteArray which contains our
	// xmlFiles content into 'channel' which we defined above
	var rss Rss
	err = xml.Unmarshal(byteValue, &rss)
	if err != nil {
		w.log.Fatalf("Parsing XML failed: %v", err)
	}

	w.log.Infof("Successfully parsed, channels: %d", len(rss.Channels))

	if w.channel == nil {
		w.log.Info("Created the very first channel")
		w.channel = &rss.Channels[0]
	} else {
		w.channel.Items = append(w.channel.Items, rss.Channels[0].Items...)
		w.log.Infof("Parsed channel addex to existing channel, new channel size is %d", len(w.channel.Items))
	}

	// defer the closing of our xmlFile so that we can parse it later on
	defer xmlFile.Close()

	return nil
}

func (w *WpExport) FindAttachments(item_id int) []Item {

	var result []Item

	if w.channel == nil {
		return result
	}

	ch := w.channel
	for i := 0; i < len(ch.Items); i++ {
		item := ch.Items[i]
		if item.ParentId == item_id && item.Type == "attachment" {
			result = append(result, item)
		}
	}

	return result
}

func (w *WpExport) Dump() {
	if w.channel == nil {
		fmt.Println("No data to dump")
		return
	}

	ch := w.channel
	fmt.Println("Channel Title: " + ch.Title)
	fmt.Println("Channel Description: " + ch.Description)
	for i := 0; i < len(ch.Items); i++ {
		item := ch.Items[i]

		taxonomies, err := item.GetTaxonomies()
		w.check(err)

		switch item.Type {
		case "post", "page":
			attachments := w.FindAttachments(item.Id)
			fmt.Println("---")
			fmt.Printf("%s %d %s\n", item.Name, item.Id, item.Type)
			fmt.Println("  title: " + item.Title)
			fmt.Printf("  date: %s\n", item.PostDate)
			fmt.Printf("  creator: %+v\n", item.Creator)
			fmt.Printf("  content: text of length %d\n", len(item.Content))
			fmt.Printf("  parent: %d\n", item.ParentId)
			fmt.Printf("  taxonomies: %+v\n", taxonomies)
			fmt.Printf("  attachments: %d\n", len(attachments))
			for j := 0; j < len(attachments); j++ {
				a := attachments[j]
				fmt.Printf("     %s %s %d\n", a.Name, a.Content, a.MenuOrder)
			}
		}
	}
}

func (w *WpExport) ensure_dir(path string) {
	w.log.Infof("Ensuring directory %s exists", path)
	err := os.MkdirAll(path, os.ModePerm)
	w.check(err)
}

func (w *WpExport) file_write_str(f *os.File, s string) {
	_, err := f.WriteString(s)
	w.check(err)
}

func (w *WpExport) Export() error {

	// do nothing if nothing was parsed before
	if w.channel == nil {
		fmt.Println("No data to export")
		return nil
	}

	hugo_root := filepath.Join(".", "build")
	w.ensure_dir(hugo_root)

	hugo_content := filepath.Join(hugo_root, "content")
	w.ensure_dir(hugo_content)

	hugo_posts := filepath.Join(hugo_content, "posts")
	w.ensure_dir(hugo_posts)

	hugo_pages := filepath.Join(hugo_content, "pages")
	w.ensure_dir(hugo_pages)

	ch := w.channel
	for i := 0; i < len(ch.Items); i++ {
		item := ch.Items[i]

		// skip media, custom types, etc.
		if item.Type != "post" && item.Type != "page" {
			continue
		}

		// Build Front Matter
		front_matter := HugoFrontMatter{}
		front_matter.Title = item.Title
		front_matter.Date = item.PostDate.Format("2006-02-01")

		taxonomies, err := item.GetTaxonomies()
		w.check(err)

		if len(taxonomies["tags"]) > 0 {
			front_matter.Tags = taxonomies["tags"]
		}

		if len(taxonomies["categories"]) > 0 {
			front_matter.Categories = taxonomies["categories"]
		}

		// filename and dir
		// get year
		var file_path string
		switch item.Type {
		case "post":
			file_dir_path := filepath.Join(hugo_posts, item.PostDate.Format("2006"))
			w.ensure_dir(file_dir_path)
			file_path = filepath.Join(hugo_posts, item.PostDate.Format("2006"), item.Name)
		case "page":
			file_path = filepath.Join(hugo_posts)
		}

		file_path = file_path + ".md"

		w.log.Infof("File to be generated: %s", file_path)

		front_matter_bytes, err := yaml.Marshal(&front_matter)
		w.check(err)

		f, err := os.Create(file_path)
		w.check(err)

		// It’s idiomatic to defer a Close immediately after opening a file.
		defer f.Close()

		w.file_write_str(f, "---\n")
		_, err = f.Write(front_matter_bytes)
		w.check(err)
		w.file_write_str(f, "---\n\n")

		// process item content
		converter := md.NewConverter("", true, nil)

		content_markdown, err := converter.ConvertString(item.Content)
		w.check(err)

		w.file_write_str(f, content_markdown)
	}

	return nil
}
