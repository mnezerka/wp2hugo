# wp2hugo

Wordpress to Hugo converter

## Features

* process multiple export files - allows to export from WP by pieces if one big export is too large
* featured image is checked against attachments of given item and marked both in
  front header param `featured_image` as well as in resources
* hierarchy for posts based on date
* hierarchy for pages based on parent relations
* some basic fixing of links to media or other location
* store comments as yaml files in resources