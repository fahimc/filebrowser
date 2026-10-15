// Gateway pages are diagnostics, not messages to display in the application.
export function httpErrorMessage(status, body='', contentType='') {
 const messages={
  0:'The connection was interrupted. Keep this page open and retry when Workspace is reachable.',
  401:'Your session expired. Sign in to FileBrowser again, then retry.',
  403:'Your account cannot access this folder or perform this action. Choose another folder or ask the Workspace owner for access.',
  409:'A file with this name already exists. Refresh the library to check whether the previous attempt completed. No existing file was overwritten.',
  413:'This upload exceeds the server or gateway size limit. Use a smaller file or ask the Workspace owner to review the limit.',
  429:'Too many requests. Wait a moment, then retry.',
  502:'Workspace could not be reached through the gateway (HTTP 502). It may be restarting or its secure connection may be interrupted. Keep this page open and retry when Workspace is available.',
  503:'Workspace is starting or temporarily unavailable (HTTP 503). Wait for startup to finish, then retry.',
  504:'The upload connection timed out (HTTP 504). Keep this page open and retry the remaining uploads.',
  521:'The gateway cannot reach Workspace (HTTP 521). Check Workspace startup and its secure connection, then retry.',
  522:'The gateway connection timed out (HTTP 522). Check that Workspace is running, then retry.',
  523:'The gateway cannot reach the Workspace server (HTTP 523). Check its secure connection, then retry.',
  524:'The server took too long to answer (HTTP 524). Keep this page open and retry the remaining uploads.',
 };
 if(messages[status])return messages[status];
 let text=typeof body==='string'?body:'';
 const isHTML=value=>/<\s*(?:!doctype|[a-z][\w:-]*)(?:\s|>|\/)/i.test(value);
 if(contentType.toLowerCase().includes('text/html')||isHTML(text))return `Workspace returned an error page (HTTP ${status}). Check that Workspace is running, then retry.`;
 try{const data=JSON.parse(text);text=typeof data.message==='string'?data.message:typeof data.error==='string'?data.error:'';}catch{/* The backend may return a short plain-text error. */}
 if(isHTML(text))return `Request failed (HTTP ${status}). Please retry.`;
 text=text.replace(/[\u0000-\u001f\u007f]/g,' ').replace(/\s+/g,' ').trim();
 return text?text.slice(0,400):`Request failed (HTTP ${status}). Please retry.`;
}
