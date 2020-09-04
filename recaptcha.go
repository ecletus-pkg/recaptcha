package recaptcha

import (
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"

	"github.com/moisespsena-go/xroute"

	"github.com/ecletus/render"
	"github.com/moisespsena/template/html/template"

	"github.com/ecletus/ecletus"
	"github.com/moisespsena-go/logging"
	path_helpers "github.com/moisespsena-go/path-helpers"

	"github.com/moisespsena-go/maps"
	"github.com/moisespsena-go/recaptcha"

	"github.com/ecletus/core"
	"github.com/ecletus/plug"
)

const CfgKey = "recaptcha"
const (
	SiteKey key = iota
	FormsKey
)

type (
	Forms = maps.MapSiSlice

	key uint8
)

var log = logging.GetOrCreateLogger(path_helpers.GetCalledDir())

type Plugin struct {
	RenderKey,
	SitesRegisterKey string
	Forms Forms
}

func (p *Plugin) RequireOptions() []string {
	return []string{p.RenderKey, p.SitesRegisterKey}
}

func (p *Plugin) Init(opts *plug.Options) {
	ecl := opts.GetInterface(ecletus.ECLETUS).(*ecletus.Ecletus)
	var cfg Config
	if err := ecl.ConfigDir.Load(&cfg, "recaptcha.yml"); err != nil && !os.IsNotExist(err) {
		log.Error(err)
		return
	}
	if !cfg.Disabled {
		if len(cfg.Forms) > 0 {
			p.Forms = append(p.Forms, cfg.Forms)
		}
	}
	for _, f := range p.Forms {
		for ouri, value := range f {
			if parts := strings.Split(ouri, ","); len(parts) > 1 {
				delete(f, ouri)
				for _, uri := range parts {
					if uri == "" {
						continue
					}

					uri = strings.TrimSpace(uri)
					if value == nil {
						value = !cfg.Disabled
					}
					f.Set(uri, value)
				}
			}
		}
	}
	register := opts.GetInterface(p.SitesRegisterKey).(*core.SitesRegister)
	register.OnAdd(func(site *core.Site) {
		Setup(site, p.Forms)
	})

	PageSetup(opts.GetInterface(p.RenderKey).(*render.Render))
}

func PageSetup(ph render.PageHandler) {
	ph.GetScriptHandlers().Append(&render.ScriptHandler{
		Name: "recaptcha",
		Handler: func(state *template.State, ctx *core.Context, w io.Writer) (err error) {
			if strings.Contains(ctx.Request.Host, "localhost") {
				return nil
			}
			formsI, ok := Get(ctx.Site).Data.Get(FormsKey)
			if !ok {
				return
			}
			forms := formsI.(*Forms)
			if forms.Has(ctx.Request.URL.Path[1:]) {
				w.Write([]byte(Get(ctx.Site).Site.HeaderScript()))
			}
			return
		},
	})

	ph.GetStyleHandlers().Append(&render.StyleHandler{
		Name: "recaptcha",
		Handler: func(state *template.State, ctx *core.Context, w io.Writer) (err error) {
			if strings.Contains(ctx.Request.Host, "localhost") {
				return nil
			}
			formsI, ok := Get(ctx.Site).Data.Get(FormsKey)
			if !ok {
				return
			}
			forms := formsI.(*Forms)
			if forms.Has(ctx.Request.URL.Path[1:]) {
				w.Write([]byte(Get(ctx.Site).Site.HeaderStyle()))
			}
			return
		},
	})

	ph.GetFormHandlers().Append(&render.FormHandler{
		Name: "recaptcha",
		Handler: func(state *render.FormState, ctx *core.Context) (err error) {
			if strings.Contains(ctx.Request.Host, "localhost") {
				return nil
			}

			formsI, ok := Get(ctx.Site).Data.Get(FormsKey)
			if !ok {
				return
			}
			state.Body = strings.TrimSpace(state.Body)
			forms := formsI.(*Forms)
			pth := ctx.Request.URL.Path[1:]
			if mi, ok := forms.Get(pth); ok {
				if m, ok := mi.(map[interface{}]interface{}); ok {
					if v, ok := m[state.Name]; ok && v.(bool) {
						Recaptcha := Get(ctx.Site)
						state.Body = Recaptcha.Site.Script(state.Name, state.Body)
					} else if v, ok := m["*"]; ok && v.(bool) {
						Recaptcha := Get(ctx.Site)
						var action = state.Name
						if actionPos := strings.Index(state.Body, `action="`); actionPos > 0 {
							actionPos+=8
							end := strings.IndexByte(state.Body[actionPos:], '"')
							action = state.Body[actionPos : actionPos+end]
							if u, err := url.Parse(action); err == nil {
								action = u.Path
							}
						}
						state.Body = Recaptcha.Site.Script(action, state.Body)
					}
				} else if v, ok := mi.(bool); ok && v {
					Recaptcha := Get(ctx.Site)
					state.Body = Recaptcha.Site.Script(state.Name, state.Body)
				}
			}
			return nil
		},
	})
}

func Setup(site *core.Site, forms Forms) {
	var cfg maps.MapSI
	if !cfg.ReadKey(site.Config(), CfgKey) {
		return
	}
	if siteForms, ok := cfg.GetMapS("forms"); ok {
		forms.Append(siteForms)
	}
	Recaptcha := recaptcha.New(cfg["private_key"].(string), cfg["site_key"].(string))
	Recaptcha.SkipFunc = func(r *http.Request) bool {
		if _, ok := forms.GetMap(r.URL.Path[1:]); ok {
			return false
		}
		return true
	}
	Recaptcha.FailedFunc = recaptcha.DefaultValidateFailedHandler
	Recaptcha.Data.Set(FormsKey, &forms)
	site.Data.Set(SiteKey, Recaptcha)
	site.Middlewares.Add(&xroute.Middleware{
		Name: "recaptcha",
		Handler: func(chain *xroute.ChainHandler) {
			r := chain.Request()
			if r.Form != nil {
				switch r.Method {
				case http.MethodPost, http.MethodPut:
					if ok, _ := Recaptcha.Validate(chain.Writer, r); !ok {
						return
					}
				}
			}
			chain.Next()
		},
	})
}

func Get(site *core.Site) (r *recaptcha.ReCaptcha) {
	if i, ok := site.Data.Get(SiteKey); ok {
		return i.(*recaptcha.ReCaptcha)
	}
	return
}
