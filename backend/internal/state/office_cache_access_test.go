package state

import (
 "os"
 "path/filepath"
 "testing"
 "github.com/gtsteffaniak/filebrowser/backend/internal/utils"
 "github.com/gtsteffaniak/filebrowser/backend/pkg/settings"
)
func TestOfficeRecoveryCacheIsNeverExposed(t *testing.T){
 previous:=settings.Config.Server.CacheDir;defer func(){settings.Config.Server.CacheDir=previous}()
 source:=t.TempDir();settings.Config.Server.CacheDir=filepath.Join(source,"cache")
 dir:=filepath.Join(settings.Config.Server.CacheDir,"office-sessions");if err:=os.MkdirAll(dir,0700);err!=nil{t.Fatal(err)}
 os.WriteFile(filepath.Join(dir,"private.json"),[]byte("private recovery data"),0600)
 for _,name:=range []string{"alice","admin",""}{
  if AccessPermitted(source,utils.IndexPathFromNormalized("/cache/office-sessions/private.json",false),name){t.Fatalf("recovery data exposed to %q",name)}
 }
}
