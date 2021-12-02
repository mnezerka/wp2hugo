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
	"regexp"
	"strconv"
	"strings"

	md "github.com/JohannesKaufmann/html-to-markdown"
	"github.com/op/go-logging"
	"gopkg.in/yaml.v2"
)

const ITEM_IMAGES_DIR = "images"

type WpExport struct {
	log     *logging.Logger
	channel *Channel

	hugo_root    string
	hugo_content string
	hugo_posts   string
	hugo_pages   string

	ConfigNoDownloads bool
	ConfigNoComments  bool
	ConfigOutputDir   string
}

func NewWpExport(logger *logging.Logger) *WpExport {
	wp_export := WpExport{}
	wp_export.log = logger
	wp_export.ConfigNoDownloads = false
	wp_export.ConfigNoComments = false
	wp_export.ConfigOutputDir = "build"

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

func (w *WpExport) FindItem(item_id int) *Item {

	var result *Item = nil

	if w.channel == nil {
		return result
	}

	ch := w.channel
	for i := 0; i < len(ch.Items); i++ {
		item := ch.Items[i]
		if item.Id == item_id {
			result = &item
			break
		}
	}

	return result
}

func (w *WpExport) FindParentItems(item *Item) []*Item {

	var result []*Item

	parent_id := item.ParentId
	for parent_id != 0 {
		parent := w.FindItem(parent_id)
		if parent == nil {
			w.log.Fatalf("Invalid parent hierarchy for page %s (%d)", item.Title, item.Id)
		}
		result = append(result, parent)
		parent_id = parent.ParentId
	}

	return result
}

func (w *WpExport) ensure_dir(path string) {
	w.log.Debugf("Ensuring directory %s exists", path)
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

		w.prepareItemFeaturedImage(&item, &front_matter)

		// first check if list index already exists in item directory
		// (as a result of previous hierarchy conversions). If exists, use it
		// instead of page bundle file
		index_file := "index.md"
		if _, err := os.Stat(filepath.Join(item_dir, "_index.md")); err == nil {
			w.log.Debugf("List index file already exists in %d, keeping existing index", item_dir)
			index_file = "_index.md"
		}

		file_path := filepath.Join(item_dir, index_file)
		w.writeItem(&item, &front_matter, file_path)

		file_path = filepath.Join(item_dir, "comments.yaml")
		w.writeItemComments(&item, file_path)
	}

	return nil
}

func (w *WpExport) prepareDirs() {

	w.hugo_root = filepath.Join(w.ConfigOutputDir)
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
		file_path = w.hugo_pages

		// look for parent pages and build appropriate directory hierarchy
		// including _index.md files to properly configure page lists and bundles
		parents := w.FindParentItems(item)

		// if some parents exists for given page
		if len(parents) > 0 {
			// loop through parents top down
			for i := len(parents) - 1; i >= 0; i-- {

				// ensure dir
				file_path = filepath.Join(file_path, parents[i].Name)
				w.ensure_dir(file_path)

				// create list index file
				if _, err := os.Stat(filepath.Join(file_path, "index.md")); err == nil {
					// rename index.md to _index.md
					os.Rename(filepath.Join(file_path, "index.md"), filepath.Join(file_path, "_index.md"))
					w.log.Infof("Renaming %s to $s", filepath.Join(file_path, "index.md"), filepath.Join(file_path, "_index.md"))
				} else {
					w.touchFile(filepath.Join(file_path, "_index.md"))
				}
			}
		}

		file_path = filepath.Join(file_path, item.Name)
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
		file_ext := strings.ToLower(filepath.Ext(file_name))
		target_file_name := strings.ToLower(file_name)

		w.log.Debugf("Processing attachment %s", file_name)

		switch file_ext {
		case ".jpg", ".jpeg", ".png", ".gif":

			r := HugoFrontMatterResource{
				Src:    filepath.Join(ITEM_IMAGES_DIR, target_file_name),
				Title:  a.Content,
				Params: make(map[string]interface{}),
			}

			target_dir := filepath.Join(item_dir, ITEM_IMAGES_DIR)
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

		case ".pdf":

			target_dir := filepath.Join(item_dir, "docs")
			w.ensure_dir(target_dir)

			target_file_path := filepath.Join(target_dir, target_file_name)

			// fetch file and store it
			w.downloadFile(a.AttachmentUrl, target_file_path)

		default:
			w.log.Warningf("Unknown attachment type %s (%s)", file_name, a.AttachmentUrl)
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

	w.log.Debugf("Writing item data to file: %s", file_path)

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

	// fix all image links
	content_markdown = w.fixLinks(content_markdown)

	w.file_write_str(f, content_markdown)
}

func (w *WpExport) writeItemComments(item *Item, file_path string) {

	w.log.Debugf("Writing item comments data to file: %s", file_path)

	// if comments sould be added
	if w.ConfigNoComments {
		return
	}

	if len(item.Comments) == 0 {
		return
	}

	item.Comments = w.buildCommentsTree(item.Comments, 0)

	comments_bytes, err := yaml.Marshal(item.Comments)
	w.check(err)

	f, err := os.Create(file_path)
	w.check(err)
	// It’s idiomatic to defer a Close immediately after opening a file.
	defer f.Close()
	_, err = f.Write(comments_bytes)
	w.check(err)
}

func (w *WpExport) touchFile(file_path string) {

	// check if local file exists
	if _, err := os.Stat(file_path); err == nil {
		return
	}

	// file doesn't exist, let's create it
	f, err := os.Create(file_path)
	w.check(err)

	// It’s idiomatic to defer a Close immediately after opening a file.
	defer f.Close()
}

// DownloadFile will download a url to a local file. It's efficient because it will
// write as it downloads and not load the whole file into memory.
func (w *WpExport) downloadFile(url string, file_path string) {

	// check if local file exists
	if _, err := os.Stat(file_path); err == nil {
		w.log.Debugf("File %s exists, keeping existing content (no overwrite)", file_path)
		return
	}

	if w.ConfigNoDownloads {
		w.log.Debugf("Skipping download of file %s due to --no-dowloads flag", file_path)
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

// look for featured image in item metadata
// must be called afther attachements are converted into item resources
func (w *WpExport) prepareItemFeaturedImage(item *Item, fm *HugoFrontMatter) {
	for i := 0; i < len(item.Meta); i++ {
		if item.Meta[i].Key == "_thumbnail_id" {
			// we have media id, look for the appropriate item
			int_value, err := strconv.Atoi(item.Meta[i].Value)
			w.check(err)
			featured_image_item := w.FindItem(int_value)
			// if media item was found
			if featured_image_item != nil {
				// consturct file path as is used in both front header resources and post/page bundles
				file_name := filepath.Join(ITEM_IMAGES_DIR, strings.ToLower(path.Base(featured_image_item.AttachmentUrl)))
				// look for file name in current item attachments, this is necessary check
				// since we cannot convert references to media that are not attached to given item
				// it is possible, but it will require to fetch one more additional image and put it into resources
				for j := 0; j < len(fm.Resources); j++ {
					if fm.Resources[j].Src == file_name {
						// we finally identified valid featured image

						// let's store it in item parameter
						fm.FeaturedImage = file_name

						// and also mark appripriate resource by param
						fm.Resources[j].Params["featured"] = true
						break
					}
				}
			}

			break
		}
	}
}

func (w *WpExport) buildCommentsTree(comments []ItemComment, parent_id int) []ItemComment {

	var result []ItemComment

	for i := 0; i < len(comments); i++ {
		c := comments[i]

		// skip items that are not children of parent_id
		if c.ParentId != parent_id {
			continue
		}

		c.Comments = w.buildCommentsTree(comments, c.Id)

		result = append(result, c)
	}

	return result
}

func (w *WpExport) fixLinks(md string) string {

	url := regexp.QuoteMeta(w.channel.Link)

	// All image links have the following form:
	// [![](https://some.domain/wp-content/uploads/2005/3849/filename.jpg)](https://some.domain/wp-content/uploads/2005/3849/filename.jpg)
	fix_images := regexp.MustCompile(`\[!\[\]\([^)]+\)\]\(` + url + `/wp-content/.*/([^./)]+\.[[:alpha:]]+)\)`)
	md = fix_images.ReplaceAllString(md, "{{<figure src=\"images/$1\">}}")

	// All simple image links have the following form:
	// ![](https://some.domain/wp-content/uploads/kolo_prumer_diagram.png)
	fix_images_simple := regexp.MustCompile(`!\[\]\(` + url + `/wp-content/.*/([^./)]+\.[[:alpha:]]+)\)`)
	md = fix_images_simple.ReplaceAllString(md, "{{<figure src=\"images/$1\">}}")

	// All media links have the following form:
	// [Eustachova chata](https://some.domain/wp-content/uploads/file.pdf)
	fix_media_links := regexp.MustCompile(`\[([^]]+)\]\(` + url + `/wp-content/[^)]*/([^./)]+\.[[:alpha:]]+)\)`)
	md = fix_media_links.ReplaceAllString(md, `[$1]({{<ref "/docs/$2" >}})`)

	// All category links have the following form:
	// [music](https://some.domain/category/music/)
	fix_cat_links := regexp.MustCompile(`\[([^]]+)\]\(` + url + `/category/(.*)\)`)
	md = fix_cat_links.ReplaceAllString(md, `[$1]({{<ref "/categories/$2" >}})`)

	// All links have the following form:
	// [something](https://some.domain/path/path/path/)
	fix_links := regexp.MustCompile(`\[([^]]+)\]\(` + url + `/(.*)\)`)
	md = fix_links.ReplaceAllString(md, `[$1]({{<ref "/$2" >}})`)

	return md
}
