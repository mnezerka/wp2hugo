package wordpress

import (
	"encoding/xml"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"

	md "github.com/JohannesKaufmann/html-to-markdown"
	"github.com/op/go-logging"
	"gopkg.in/yaml.v2"
)

type WpExport struct {
	log     *logging.Logger
	channel *Channel

	hugo_root    string
	hugo_content string
	hugo_posts   string
	hugo_pages   string
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

	w.prepareDirs()

	ch := w.channel
	for i := 0; i < len(ch.Items); i++ {
		item := ch.Items[i]

		// skip media, custom types, etc.
		if item.Type != "post" && item.Type != "page" {
			continue
		}

		// get dir
		item_dir := w.prepareItemDir(&item)

		// Build Front Matter
		front_matter := HugoFrontMatter{}
		front_matter.Title = item.Title
		front_matter.Date = item.PostDate.Format("2006-01-02")
		front_matter.Slug = item.Name

		w.prepareItemTaxonomies(&item, &front_matter)

		w.prepareItemAttachments(&item, &front_matter, item_dir)

		file_path := filepath.Join(item_dir, "index.md")
		w.writeItem(&item, &front_matter, file_path)
	}

	return nil
}

func (w *WpExport) prepareDirs() {

	w.hugo_root = filepath.Join(".", "build")
	w.ensure_dir(w.hugo_root)

	w.hugo_content = filepath.Join(w.hugo_root, "content")
	w.ensure_dir(w.hugo_content)

	w.hugo_posts = filepath.Join(w.hugo_content, "posts")
	w.ensure_dir(w.hugo_posts)

	w.hugo_pages = filepath.Join(w.hugo_content, "pages")
	w.ensure_dir(w.hugo_pages)
}

func (w *WpExport) prepareItemDir(item *Item) string {

	// construct file path, starting with proper dir
	var file_path string
	switch item.Type {
	case "post":
		// create directory derived from post date
		file_path = filepath.Join(w.hugo_posts, item.PostDate.Format("2006"))
		file_path = filepath.Join(file_path, item.PostDate.Format("2006_01_02_")+item.Name)

	case "page":
		file_path = filepath.Join(w.hugo_pages, item.Name)
	}

	// create single directory for each post/page since we need a bundle (to
	// be able to store attachments)
	w.ensure_dir(file_path)

	return file_path
}

func (w *WpExport) prepareItemAttachments(item *Item, fh *HugoFrontMatter, item_dir string) {
	attachments := w.FindAttachments(item.Id)

	for i := 0; i < len(attachments); i++ {
		a := attachments[i]
		file_name := path.Base(a.AttachmentUrl)
		file_ext := filepath.Ext(file_name)
		target_file_name := strings.ToLower(file_name)

		w.log.Infof("Processing attachment %s", file_name)

		switch file_ext {
		case ".jpg", ".jpeg", ".png":

			r := HugoFrontMatterResource{
				Src:    filepath.Join("images", target_file_name),
				Title:  a.Content,
				Params: make(map[string]interface{}),
			}

			target_dir := filepath.Join(item_dir, "images")
			w.ensure_dir(target_dir)

			target_file_path := filepath.Join(target_dir, target_file_name)

			// fetch file and store it
			w.downloadFile(a.AttachmentUrl, target_file_path)

			r.Params["weight"] = a.MenuOrder
			fh.Resources = append(fh.Resources, r)

		case ".gpx":

			target_dir := filepath.Join(item_dir, "gpx")
			w.ensure_dir(target_dir)

			target_file_path := filepath.Join(target_dir, target_file_name)

			// fetch file and store it
			w.downloadFile(a.AttachmentUrl, target_file_path)
		default:
			w.log.Warningf("Unknow attachment type %s (%s)", file_name, a.AttachmentUrl)
		}

	}
}

func (w *WpExport) prepareItemTaxonomies(item *Item, fm *HugoFrontMatter) {

	taxonomies, err := item.GetTaxonomies()
	w.check(err)

	if len(taxonomies["tags"]) > 0 {
		fm.Tags = taxonomies["tags"]
	}

	if len(taxonomies["categories"]) > 0 {
		fm.Categories = taxonomies["categories"]
	}

}

func (w *WpExport) writeItem(item *Item, fm *HugoFrontMatter, file_path string) {

	w.log.Infof("Writing item data to file: %s", file_path)

	front_matter_bytes, err := yaml.Marshal(fm)
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

// DownloadFile will download a url to a local file. It's efficient because it will
// write as it downloads and not load the whole file into memory.
func (w *WpExport) downloadFile(url string, file_path string) {

	// check if local file exists
	if _, err := os.Stat(file_path); err == nil {
		w.log.Warningf("File %s exists, keeping existing content (no overwrite)", file_path)
		return
	}

	resp, err := http.Get(url)
	w.check(err)

	defer resp.Body.Close()

	// Create the file
	out, err := os.Create(file_path)
	w.check(err)
	defer out.Close()

	// Write the body to file
	_, err = io.Copy(out, resp.Body)
	w.check(err)
}
