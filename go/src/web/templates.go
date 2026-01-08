package web

import (
	"fmt"
	"html/template"
	"path/filepath"

	"github.com/nhirsama/Goster-IoT/src/inter"
)

// loadTemplates 加载所有 HTML 模板
func loadTemplates(htmlDir string) map[string]*template.Template {
	templates := make(map[string]*template.Template)

	funcMap := template.FuncMap{
		"dict": func(values ...interface{}) (map[string]interface{}, error) {
			if len(values)%2 != 0 {
				return nil, fmt.Errorf("invalid dict call")
			}
			dict := make(map[string]interface{}, len(values)/2)
			for i := 0; i < len(values); i += 2 {
				key, ok := values[i].(string)
				if !ok {
					return nil, fmt.Errorf("dict keys must be strings")
				}
				dict[key] = values[i+1]
			}
			return dict, nil
		},
		"hasPerm": func(userPerm int, reqPerm int) bool {
			return userPerm >= reqPerm
		},
		"permType": func(p int) inter.PermissionType {
			return inter.PermissionType(p)
		},
	}

	parse := func(name, file string) *template.Template {
		t := template.New(name).Funcs(funcMap)
		t = template.Must(t.ParseFiles(filepath.Join(htmlDir, file)))
		return t
	}

	templates["index.html"] = parse("index.html", "index.html")
	templates["login.html"] = parse("login.html", "login.html")
	templates["register.html"] = parse("register.html", "register.html")
	templates["device_list.html"] = parse("device_list.html", "device_list.html")
	templates["metrics.html"] = parse("metrics.html", "metrics.html")
	templates["pending_list.html"] = parse("pending_list.html", "pending_list.html")
	templates["blacklist.html"] = parse("blacklist.html", "blacklist.html")
	templates["user_list.html"] = parse("user_list.html", "user_list.html")
	templates["pending_table.html"] = parse("pending_table.html", "pending_table.html")
	templates["blacklist_table.html"] = parse("blacklist_table.html", "blacklist_table.html")
	return templates
}
