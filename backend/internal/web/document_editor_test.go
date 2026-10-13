package web

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"testing/fstest"

	"github.com/gtsteffaniak/filebrowser/backend/pkg/settings"
)

func TestDocumentEditorRoutes(t *testing.T) {
	previousAssets, previousEditor, previousDev := assetFs, documentEditorFs, settings.Env.IsDevMode
	t.Cleanup(func() {
		assetFs, documentEditorFs, settings.Env.IsDevMode = previousAssets, previousEditor, previousDev
	})
	settings.Env.IsDevMode = false
	assetFs = fstest.MapFS{
		"ranuts-document/editor.html":                       {Data: []byte("<html>Office editor</html>")},
		"ranuts-document/assets/editor.js":                  {Data: []byte("editor bundle")},
		"ranuts-document/ran-tokens.1234.css":               {Data: []byte("body{color:blue}")},
		"ranuts-document/fonts/001":                         {Data: []byte("font bytes")},
		"ranuts-document/sdkjs/common/wasm/x2t/x2t.wasm.br": {Data: []byte("compressed wasm")},
	}
	for _, base := range []string{"/", "/drive/"} {
		t.Run(base, func(t *testing.T) {
			router := http.NewServeMux()
			registerDocumentEditorRoutes(router, base)
			for _, path := range []string{base + "editor", "/assets/editor.js", base + "assets/editor.js", "/ran-tokens.1234.css", base + "ran-tokens.1234.css", "/fonts/001"} {
				response := httptest.NewRecorder()
				router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, path, nil))
				if response.Code != http.StatusOK || response.Body.Len() == 0 {
					t.Errorf("%s: status=%d, body length=%d", path, response.Code, response.Body.Len())
				}
			}
			response := httptest.NewRecorder()
			router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/sdkjs/common/wasm/x2t/x2t.wasm.br", nil))
			if response.Header().Get("Content-Encoding") != "br" || response.Header().Get("Content-Type") != "application/wasm" {
				t.Errorf("incorrect WASM transport headers: %v", response.Header())
			}
			head := httptest.NewRecorder()
			router.ServeHTTP(head, httptest.NewRequest(http.MethodHead, base+"assets/editor.js", nil))
			if head.Code != http.StatusOK || head.Body.Len() != 0 || head.Header().Get("Content-Length") != "13" {
				t.Errorf("incorrect HEAD asset response: status=%d, headers=%v, body=%q", head.Code, head.Header(), head.Body.String())
			}
			missing := httptest.NewRecorder()
			router.ServeHTTP(missing, httptest.NewRequest(http.MethodGet, "/assets/missing.js", nil))
			if missing.Code != http.StatusNotFound {
				t.Errorf("missing asset status: %d", missing.Code)
			}
		})
	}
}
