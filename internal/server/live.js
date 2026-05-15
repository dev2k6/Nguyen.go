// Nguyen.go Live Mode client runtime.
//
// Connects a WebSocket to /_nguyen/live/<route>, applies server-pushed
// VDOM patches to the live DOM, and forwards user events back. The
// runtime is intentionally tiny — there is no virtual DOM on the
// client, no diffing, no state. The server is the source of truth.
//
// Mount: place a single <div data-nguyen-live="<route>"> in the
// server-rendered HTML. The runtime finds it on load, opens a
// WebSocket and starts the session.

(function () {
  'use strict';

  if (window.__nguyenLiveStarted) return;
  window.__nguyenLiveStarted = true;

  document.addEventListener('DOMContentLoaded', boot);

  function boot() {
    var mounts = document.querySelectorAll('[data-nguyen-live]');
    for (var i = 0; i < mounts.length; i++) {
      attach(mounts[i]);
    }
  }

  function attach(root) {
    var route = root.getAttribute('data-nguyen-live');
    var url = wsURL('/_nguyen/live' + (route.charAt(0) === '/' ? route : '/' + route));
    var session = new LiveSession(root, url);
    root.__live = session;
    session.connect();
  }

  function wsURL(path) {
    var proto = location.protocol === 'https:' ? 'wss:' : 'ws:';
    return proto + '//' + location.host + path;
  }

  function LiveSession(root, url) {
    this.root = root;
    this.url = url;
    this.socket = null;
    this.attempt = 0;
    this.handlers = bindEventDelegation(this);
  }

  LiveSession.prototype.connect = function () {
    var self = this;
    try {
      this.socket = new WebSocket(this.url);
    } catch (e) {
      console.warn('[live] cannot open socket', e);
      this.scheduleReconnect();
      return;
    }
    this.socket.addEventListener('open', function () {
      self.attempt = 0;
    });
    this.socket.addEventListener('message', function (e) {
      try {
        var msg = JSON.parse(e.data);
        self.onMessage(msg);
      } catch (err) {
        console.warn('[live] bad frame', err, e.data);
      }
    });
    this.socket.addEventListener('close', function () { self.scheduleReconnect(); });
    this.socket.addEventListener('error', function () {});
  };

  LiveSession.prototype.scheduleReconnect = function () {
    var self = this;
    var delay = Math.min(30000, 250 * Math.pow(2, this.attempt++));
    setTimeout(function () { self.connect(); }, delay);
  };

  LiveSession.prototype.send = function (obj) {
    if (!this.socket || this.socket.readyState !== 1) return;
    try {
      this.socket.send(JSON.stringify(obj));
    } catch (e) {
      console.warn('[live] send failed', e);
    }
  };

  LiveSession.prototype.onMessage = function (msg) {
    switch (msg.t) {
      case 'replace': this.applyReplace(msg.html); break;
      case 'diff':    this.applyDiff(msg.ops); break;
      case 'error':   console.warn('[live] server error:', msg.error); break;
      default:        console.warn('[live] unknown frame', msg);
    }
  };

  LiveSession.prototype.applyReplace = function (html) {
    this.root.innerHTML = html;
  };

  LiveSession.prototype.applyDiff = function (ops) {
    for (var i = 0; i < ops.length; i++) {
      this.applyOp(ops[i]);
    }
  };

  LiveSession.prototype.applyOp = function (op) {
    var target = nodeAtPath(this.root, op.path || '');
    if (!target && op.op !== 'root' && op.op !== 'insert') {
      console.warn('[live] missing path', op.path);
      return;
    }
    switch (op.op) {
      case 'root':
        this.root.innerHTML = op.html || '';
        break;
      case 'replace':
        if (target && target.outerHTML !== undefined) {
          target.outerHTML = op.html || '';
        }
        break;
      case 'text':
        if (target && target.nodeType === 1) {
          target.textContent = op.value || '';
        } else if (target) {
          target.nodeValue = op.value || '';
        }
        break;
      case 'attr':
        if (target && target.setAttribute) {
          if (op.attrs) {
            for (var k in op.attrs) {
              if (Object.prototype.hasOwnProperty.call(op.attrs, k)) {
                target.setAttribute(k, op.attrs[k]);
              }
            }
          }
        }
        break;
      case 'insert':
        if (op.html) {
          var parent = nodeAtPath(this.root, parentPath(op.path)) || this.root;
          var temp = document.createElement('template');
          temp.innerHTML = op.html;
          parent.appendChild(temp.content.firstElementChild || temp.content);
        }
        break;
      case 'remove':
        if (target && target.parentNode) {
          target.parentNode.removeChild(target);
        }
        break;
      default:
        console.warn('[live] unknown op', op.op);
    }
  };

  function nodeAtPath(root, path) {
    if (!path) return root;
    var parts = path.split('/');
    var node = root;
    for (var i = 0; i < parts.length; i++) {
      var idx = parseInt(parts[i], 10);
      if (!node || !node.childNodes || idx >= node.childNodes.length) return null;
      node = node.childNodes[idx];
    }
    return node;
  }

  function parentPath(p) {
    if (!p) return '';
    var idx = p.lastIndexOf('/');
    return idx < 0 ? '' : p.substring(0, idx);
  }

  function bindEventDelegation(session) {
    var events = ['click', 'submit', 'input', 'change', 'keydown', 'keyup', 'focus', 'blur'];
    function dispatch(e) {
      var attr = '@' + e.type;
      var node = e.target;
      while (node && node !== session.root.parentNode) {
        var name = node.getAttribute && node.getAttribute(attr);
        if (name) {
          if (e.type === 'submit') e.preventDefault();
          if (e.type === 'click' && node.tagName === 'A' && node.getAttribute('href')) {
            // stay on the page during live navigation
            e.preventDefault();
          }
          var value = readValue(node);
          session.send({
            t: 'event',
            name: stripCall(name),
            target: domPath(session.root, node),
            value: value,
          });
          return;
        }
        node = node.parentNode;
      }
    }
    for (var i = 0; i < events.length; i++) {
      session.root.addEventListener(events[i], dispatch, true);
    }
    return dispatch;
  }

  function stripCall(s) {
    var idx = s.indexOf('(');
    return idx < 0 ? s.trim() : s.substring(0, idx).trim();
  }

  function readValue(node) {
    if (node.tagName === 'INPUT' || node.tagName === 'TEXTAREA' || node.tagName === 'SELECT') {
      if (node.type === 'checkbox' || node.type === 'radio') return node.checked;
      return node.value;
    }
    return null;
  }

  function domPath(root, node) {
    var parts = [];
    while (node && node !== root) {
      var parent = node.parentNode;
      if (!parent) break;
      var idx = Array.prototype.indexOf.call(parent.childNodes, node);
      parts.unshift(String(idx));
      node = parent;
    }
    return parts.join('/');
  }
})();
