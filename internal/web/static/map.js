function cpMap(id) {
  var m = L.map(id);
  L.tileLayer('/tiles/{z}/{x}/{y}.png', {
    maxZoom: 19,
    attribution: '&copy; <a href="https://www.openstreetmap.org/copyright">OpenStreetMap</a> contributors'
  }).addTo(m);
  return m;
}

function cpSitesMap(id, sites) {
  var m = cpMap(id);
  var pts = [];
  (sites || []).forEach(function (s) {
    L.marker([s.Lat, s.Lon]).addTo(m).bindPopup(s.Name);
    pts.push([s.Lat, s.Lon]);
  });
  if (pts.length) { m.fitBounds(pts, { padding: [30, 30], maxZoom: 12 }); } else { m.setView([47.0, 8.5], 7); }
  return m;
}

function cpSiteFormMap(id, latInput, lonInput) {
  var m = cpMap(id);
  var lat = parseFloat(latInput.value), lon = parseFloat(lonInput.value);
  var marker = null;
  function place(ll, zoomTo) {
    if (marker) { marker.setLatLng(ll); } else {
      marker = L.marker(ll, { draggable: true }).addTo(m);
      marker.on('dragend', function () { set(marker.getLatLng()); });
    }
    if (zoomTo) {
      var z = m.getZoom();
      m.setView(ll, (z === undefined || z < 12) ? 12 : z);
    }
  }
  function set(ll) {
    latInput.value = ll.lat.toFixed(5);
    lonInput.value = ll.lng.toFixed(5);
    place(ll, false);
  }
  if (!isNaN(lat) && !isNaN(lon)) { place([lat, lon], true); } else { m.setView([47.0, 8.5], 7); }
  m.on('click', function (e) { set(e.latlng); });
  function fromInputs() {
    var la = parseFloat(latInput.value), lo = parseFloat(lonInput.value);
    if (!isNaN(la) && !isNaN(lo)) { place([la, lo], true); }
  }
  latInput.addEventListener('change', fromInputs);
  lonInput.addEventListener('change', fromInputs);
  return m;
}

function cpLinkMap(id, a, b) {
  var m = cpMap(id);
  L.marker([a.Lat, a.Lon]).addTo(m).bindPopup(a.Name);
  L.marker([b.Lat, b.Lon]).addTo(m).bindPopup(b.Name);
  L.polyline([[a.Lat, a.Lon], [b.Lat, b.Lon]], { className: 'link-path' }).addTo(m);
  m.fitBounds([[a.Lat, a.Lon], [b.Lat, b.Lon]], { padding: [30, 30] });
  return m;
}

function cpCoverageMap(id, tx, rangeM, imageUrl, bounds) {
  var m = cpMap(id);
  m.cpCircles = L.layerGroup().addTo(m);
  m.cpBase = null;
  L.marker([tx.Lat, tx.Lon]).addTo(m).bindPopup(tx.Name);
  m.fitBounds(L.latLng(tx.Lat, tx.Lon).toBounds(2 * rangeM));
  L.circle([tx.Lat, tx.Lon], { radius: rangeM, className: 'range-circle', fill: false }).addTo(m.cpCircles);
  if (imageUrl) {
    m.cpBase = L.imageOverlay(imageUrl, [[bounds.South, bounds.West], [bounds.North, bounds.East]], { opacity: 0.6 }).addTo(m);
  }
  return m;
}

function cpCoverageOverlays(m, viewUrl, compositeUrl) {
  var slider = document.getElementById('opacity');
  if (!slider) { return m; }
  var out = document.getElementById('opacity-value');
  var extra = {};
  var composite = null, compositeSrc = null, seq = 0;

  function opacity() { return slider.value / 100; }
  function apply() {
    m.eachLayer(function (l) { if (l instanceof L.ImageOverlay) { l.setOpacity(opacity()); } });
    out.textContent = slider.value + ' %';
  }
  slider.addEventListener('input', apply);
  slider.addEventListener('change', save);

  var circles = document.getElementById('circles');

  function ticked() {
    var on = [];
    document.querySelectorAll('input[data-overlay]:checked').forEach(function (c) { on.push(c.dataset.overlay); });
    return on;
  }

  function save() {
    if (!viewUrl) { return; }
    var body = new URLSearchParams({
      opacity: slider.value,
      overlays: ticked().join(','),
      circles: circles && !circles.checked ? '0' : '1'
    });
    fetch(viewUrl, { method: 'POST', body: body, headers: { 'Content-Type': 'application/x-www-form-urlencoded' } })
      .catch(function () { });
  }

  function dropComposite() {
    if (composite) { m.removeLayer(composite); composite = null; }
    if (compositeSrc) { URL.revokeObjectURL(compositeSrc); compositeSrc = null; }
  }

  function refresh() {
    var on = ticked();
    var n = ++seq;
    if (!on.length) {
      dropComposite();
      if (m.cpBase && !m.hasLayer(m.cpBase)) { m.cpBase.addTo(m); apply(); }
      return;
    }
    fetch(compositeUrl + '?with=' + on.join(','))
      .then(function (res) {
        if (!res.ok) { throw new Error('composite ' + res.status); }
        var b = res.headers.get('X-Raster-Bounds').split(',').map(Number); // S,W,N,E
        return res.blob().then(function (blob) { return { blob: blob, bounds: [[b[0], b[1]], [b[2], b[3]]] }; });
      })
      .then(function (img) {
        if (n !== seq) { return; }
        dropComposite();
        compositeSrc = URL.createObjectURL(img.blob);
        composite = L.imageOverlay(compositeSrc, img.bounds, { opacity: opacity() }).addTo(m);
        if (m.cpBase) { m.removeLayer(m.cpBase); }
      })
      .catch(function () { });
  }

  document.querySelectorAll('input[data-overlay]').forEach(function (cb) {
    function toggle() {
      var id = cb.dataset.overlay;
      if (cb.checked) {
        var b = [[+cb.dataset.south, +cb.dataset.west], [+cb.dataset.north, +cb.dataset.east]];
        var ll = [+cb.dataset.lat, +cb.dataset.lon];
        extra[id] = {
          marker: L.marker(ll).bindPopup(cb.dataset.name).addTo(m),
          circle: L.circle(ll, { radius: +cb.dataset.range, className: 'range-circle', fill: false }).addTo(m.cpCircles)
        };
        if (!m.getBounds().intersects(b)) { m.fitBounds(b); }
      } else if (extra[id]) {
        m.removeLayer(extra[id].marker);
        m.cpCircles.removeLayer(extra[id].circle);
        delete extra[id];
      }
    }
    cb.addEventListener('change', function () { toggle(); refresh(); save(); });
    if (cb.checked) { toggle(); }
  });
  if (ticked().length) { refresh(); }

  if (circles) {
    circles.addEventListener('change', function () {
      if (circles.checked) { m.addLayer(m.cpCircles); } else { m.removeLayer(m.cpCircles); }
      save();
    });
    if (!circles.checked) { m.removeLayer(m.cpCircles); }
  }

  apply();
  return m;
}

function cpJobWatch(url, el, badge) {
  var es = new EventSource(url);
  es.onmessage = function (ev) {
    var j = JSON.parse(ev.data);
    badge.className = 'badge badge-' + j.state;
    badge.textContent = j.state;
    if (j.state === 'queued') {
      el.textContent = 'Queued since ' + j.since + ', waiting for a worker.';
    } else if (j.state === 'running') {
      el.textContent = 'Computing since ' + j.since + ' on ' + j.worker + '.';
    } else {
      es.close();
      location.reload();
    }
  };
  // the server closes the stream after the final state; EventSource would reconnect forever
  es.onerror = function () { if (es.readyState === EventSource.CLOSED) { location.reload(); } };
}
