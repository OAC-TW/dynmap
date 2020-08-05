"use strict";

/*  Global class for simulating the movement of particle through a 1km wind grid

 credit: All the credit for this work goes to: https://github.com/cambecc for creating the repo:
 https://github.com/cambecc/earth. The majority of this code is directly take nfrom there, since its awesome.

 This class takes a canvas element and an array of data (1km GFS from http://www.emc.ncep.noaa.gov/index.php?branch=GFS)
 and then uses a mercator (forward/reverse) projection to correctly map wind vectors in "map space".

 The "start" method takes the bounds of the map at its current extent and starts the whole gridding,
 interpolation and animation process.
 */

var GradientCanvas = function(params) {
  var MIN_INTENSITY = params.minIntensity || 0;
  var MAX_INTENSITY = params.maxIntensity || 10;
  var OPACITY = params.opacity || 0.97;
  var reverseX = params.reverseX || false;
  var reverseY = params.reverseY || false;
  var DPX = params.dpx || 2;

  var defaulColorScale = [
    "rgba(  0,  0,  0, 0.0)", // transparent for no data
    "rgba( 36,104,180, 0.8)",
    "rgba( 60,157,194, 0.8)",
    "rgba(128,205,193, 0.8)",
    "rgba(151,218,168, 0.8)",
    "rgba(198,231,181, 0.8)",
    "rgba(238,247,217, 0.8)",
    "rgba(255,238,159, 0.8)",
    "rgba(252,217,125, 0.8)",
    "rgba(255,182,100, 0.8)",
    "rgba(252,150, 75, 0.8)",
    "rgba(250,112, 52, 0.8)",
    "rgba(245, 64, 32, 0.8)",
    "rgba(237, 45, 28, 0.8)",
    "rgba(220, 24, 32, 0.8)",
    "rgba(180,  0, 35, 0.8)"
  ];

  var colorScale = params.colorScale || defaulColorScale;
  var colorStyles;

  var interpolateFn = interpolateBI;
  if (typeof params.interpolateType === 'string') {
    interpolateFn = (params.interpolateType.toLowerCase() == 'nearestneighbor')? interpolateNN : interpolateBI;
  }
  if (typeof params.interpolateType === 'function') {
    interpolateFn = params.interpolateType;
  }

  var builder;
  var grid;
  var gridData = params.data;
  var date;
  var λ0, φ0, Δλ, Δφ, ni, nj;

  var setData = function(data) {
    grid = null;
    gridData = data;
    buildGrid(gridData, function(out) {
      grid = out;
    });
  };

  var setOptions = function(options) {
    if (options.hasOwnProperty("minIntensity"))
      MIN_INTENSITY = options.minIntensity;

    if (options.hasOwnProperty("maxIntensity"))
      MAX_INTENSITY = options.maxIntensity;

    if (options.hasOwnProperty("opacity")) OPACITY = +options.opacity;

    if (options.hasOwnProperty("dpx")) DPX = +options.dpx;

    if (options.hasOwnProperty("colorScale")) {
      colorScale = params.colorScale || defaulColorScale;
      colorStyles = gradient(colorScale);
    }
  };

  // interpolation for value
  var bilinearInterpolateVector = function(x, y, g00, g10, g01, g11) {
    var rx = 1 - x;
    var ry = 1 - y;
    var a = rx * ry,
      b = x * ry,
      c = rx * y,
      d = x * y;
    var v = g00 * a + g10 * b + g01 * c + g11 * d;
    return v;
  };

  var createBuilder = function(data) {
    if(!data.dx) data.dx = (data.lo2 - data.lo1) / (data.nx-1);
    if(!data.dy) data.dy = (data.la1 - data.la2) / (data.ny-1);
    var nx = data.nx;
    var ny = data.ny;
    return {
      header: data,
      data: function(i) {
        return data.d['v'][i];
      }
    };
  };

  var buildGrid = function(data, callback) {

    builder = createBuilder(data);
    var header = builder.header;

    λ0 = header.lo1;
    φ0 = header.la1; // the grid's origin (e.g., 0.0E, 90.0N)

    Δλ = header.dx;
    Δφ = header.dy; // distance between grid points (e.g., 2.5 deg lon, 2.5 deg lat)

    ni = header.nx;
    nj = header.ny; // number of grid points W-E and N-S (e.g., 144 x 73)

    grid = [];
    var p = 0;
    var isContinuous = Math.floor(ni * Δλ) >= 360;

    for (var j = 0; j < nj; j++) {
      var row = [];
      for (var i = 0; i < ni; i++, p++) {
//        row[i] = builder.data(p);
        if (reverseX) {
          row.unshift(builder.data(p));
        } else {
          row.push(builder.data(p));
        }
      }
      if (isContinuous) {
        // For wrapped grids, duplicate first column as last column to simplify interpolation logic
//        row.push(row[0]);
        if (reverseX) {
          row.unshift(row[row.length-1]);
        } else {
          row.push(row[0]);
        }
      }
//      grid[j] = row;
      if (reverseY) {
        grid.unshift(row);
      } else {
        grid.push(row);
      }
    }

    callback(grid);
  };

  /**
   * Get interpolated grid value from Lon/Lat position by bilinear
   * @param λ {Float} Longitude
   * @param φ {Float} Latitude
   * @param grid {2D-Array} data grid
   * @returns {Object}
   */
  function interpolateBI(λ, φ, grid) {
    if (!grid) return null;

    var i = floorMod(λ - λ0, 360) / Δλ; // calculate longitude index in wrapped range [0, 360)
    var j = (φ0 - φ) / Δφ; // calculate latitude index in direction +90 to -90

    var fi = Math.floor(i),
      ci = fi + 1;
    var fj = Math.floor(j),
      cj = fj + 1;

    var row;
    if ((row = grid[fj])) {
      var g00 = row[fi];
      var g10 = row[ci];
      if (isValue(g00) && isValue(g10) && (row = grid[cj])) {
        var g01 = row[fi];
        var g11 = row[ci];
        if (isValue(g01) && isValue(g11)) {
          // All four points found, so interpolate the value.
          return bilinearInterpolateVector(i - fi, j - fj, g00, g10, g01, g11);
        }
      }
    }
    return null;
  };

  /**
   * Get interpolated grid value from Lon/Lat position by nearest neighbor
   * @param λ {Float} Longitude
   * @param φ {Float} Latitude
   * @param grid {2D-Array} data grid
   * @returns {Object}
   */
  function interpolateNN(λ, φ, grid) {
    if (!grid) return null;

    var i = floorMod(λ - λ0, 360) / Δλ; // calculate longitude index in wrapped range [0, 360)
    var j = (φ0 - φ) / Δφ; // calculate latitude index in direction +90 to -90

    var fi = Math.round(i);
    var fj = Math.round(j);

    var row = grid[fj]
    if (row) {
      var g00 = row[fi]
      if (isValue(g00)) return g00;
    }
    return null;
  };

  /**
   * @returns {Boolean} true if the specified value is not null and not undefined.
   */
  var isValue = function(x) {
    return x !== null && x !== undefined && (typeof x === 'number');
  };

  /**
   * @returns {Number} returns remainder of floored division, i.e., floor(a / n). Useful for consistent modulo
   *          of negative numbers. See http://en.wikipedia.org/wiki/Modulo_operation.
   */
  var floorMod = function(a, n) {
    return a - n * Math.floor(a / n);
  };

  /**
   * @returns {Number} the value x clamped to the range [low, high].
   */
  var clamp = function(x, range) {
    return Math.max(range[0], Math.min(x, range[1]));
  };

  /**
   * @returns {Boolean} true if agent is probably a mobile device. Don't really care if this is accurate.
   */
  var isMobile = function() {
    return /android|blackberry|iemobile|ipad|iphone|ipod|opera mini|webos/i.test(
      navigator.userAgent
    );
  };

  var buildBounds = function(bounds, width, height) {
    var upperLeft = bounds[0];
    var lowerRight = bounds[1];
    var x = Math.round(upperLeft[0]); //Math.max(Math.floor(upperLeft[0], 0), 0);
    var y = Math.max(Math.floor(upperLeft[1], 0), 0);
    var xMax = Math.min(Math.ceil(lowerRight[0], width), width - 1);
    var yMax = Math.min(Math.ceil(lowerRight[1], height), height - 1);
    return {
      x: x,
      y: y,
      xMax: width,
      yMax: yMax,
      width: width,
      height: height
    };
  };

  var deg2rad = function(deg) {
    return (deg / 180) * Math.PI;
  };

  var invert = function(x, y) {
    var latlon = params.map.containerPointToLatLng(L.point(x, y));
    return [latlon.lng, latlon.lat];
  };

  var project = function(lat, lon) {
    var xy = params.map.latLngToContainerPoint(L.latLng(lat, lon));
    return [xy.x, xy.y];
  };

  var gradient = function (grad) {
        // create a 256x1 gradient color
        var canvas = document.createElement('canvas');
        var ctx = canvas.getContext('2d');
        var gradient = ctx.createLinearGradient(0, 0, 0, 256);

        canvas.width = 1;
        canvas.height = 256;

	var max = grad.length
        for (var i in grad) {
            gradient.addColorStop(i/max, grad[i]);
        }

        ctx.fillStyle = gradient;
        ctx.fillRect(0, 0, 1, 256);

        return ctx.getImageData(0, 0, 1, 256).data;
  };
  colorStyles = gradient(colorScale);

  var createField = function(columns, bounds, callback) {
    /**
     * @returns value at the point (x, y), or null if value is undefined at that point.
     */
    function field(x, y) {
      var column = columns[Math.round(x)];
      return (column && column[Math.round(y)]) || null;
    }

    // Frees the massive "columns" array for GC. Without this, the array is leaked (in Chrome) each time a new
    // field is interpolated because the field closure's context is leaked, for reasons that defy explanation.
    field.release = function() {
      columns = [];
    };

    callback(bounds, field, columns);
  };

  var interpolateField = function(gridInterpolateFn, bounds, extent, callback) {
    var columns = [];
    var x = bounds.x;

    function interpolateColumn(x) {
      var w = bounds.width;
      var h = bounds.height;
      var column = [];
      for (var y = bounds.y; y <= bounds.yMax; y += DPX) {
        var coord = invert(x, y);
        if (coord) {
          var λ = coord[0],
            φ = coord[1];
          if (isFinite(λ)) {
            var value = gridInterpolateFn(λ, φ, grid);
            for(var k=0; k<DPX; k++) column[y + k] = value;
          }
        }
      }
      for(var k=0; k<DPX; k++) columns[x + k] = column;
    }

    (function batchInterpolate() {
      var start = Date.now();
      while (x < bounds.width) {
        interpolateColumn(x);
        x += DPX;
        if (Date.now() - start > 500) { // not to block too long
          setTimeout(batchInterpolate, 25);
          return;
        }
      }
      createField(columns, bounds, callback);
    })();
  };

  var draw = function(bounds, field, columns) {
    //var colorStyles = gradient(colorScale);
    var intensityColorScale = function (val, out) { // map value to a style
      var idx = Math.max(
        0,
        Math.min(
          255*4,
          Math.round(((val - MIN_INTENSITY) / (MAX_INTENSITY - MIN_INTENSITY)) * 255)*4
        )
      );
      out[0] = colorStyles[idx + 0];
      out[1] = colorStyles[idx + 1];
      out[2] = colorStyles[idx + 2];
      out[3] = colorStyles[idx + 3];
    };

    var w = bounds.width;
    var h = bounds.height;
    var g = params.canvas.getContext("2d");
    var imageData = g.getImageData(0, 0, w, h);
    var px = imageData.data;

    var color = [0,0,0,0];
//console.log("[gradient]draw", bounds, w, h, px.length / 4, grid, columns);
    for(var i=0; i<w; i++) {
      var col = columns[i];
      for(var j=0; j<h; j++) {
        var k = (j*w + i) * 4;
        intensityColorScale(col[j], color);
        px[k + 0] = color[0]; // red
        px[k + 1] = color[1]; // green
        px[k + 2] = color[2]; // blue
        px[k + 3] = color[3]; // alpha
      }
    }
    g.putImageData(imageData, 0, 0);
  };



  var start = function(bounds, width, height, extent, cb) {
    var mapBounds = {
      south: deg2rad(extent[0][1]),
      north: deg2rad(extent[1][1]),
      east: deg2rad(extent[1][0]),
      west: deg2rad(extent[0][0]),
      width: width,
      height: height
    };

    stop();

    var run = function() {
console.time('[gradient]interpolateField')
      interpolateField(
        interpolateFn,
        buildBounds(bounds, width, height),
        mapBounds,
        function(bounds, field, columns) {
console.timeEnd('[gradient]interpolateField')
console.time('[gradient]draw')
          obj.field = field;
          draw(bounds, field, columns);
console.timeEnd('[gradient]draw')
          if(cb && typeof cb === 'function') cb();
      });
    };

    if (!grid) {
      // build grid
console.time('[gradient]buildGrid')
      buildGrid(gridData, function(out) {
console.timeEnd('[gradient]buildGrid')
        grid = out;
        run();
      });
    } else {
      run();
    }
  };

  var stop = function() {
    if (obj.field) obj.field.release();
  };

  var obj = {
    params: params,
    start: start,
    stop: stop,
    createField: createField,
    interpolatePoint: interpolateFn,
    setData: setData,
    setOptions: setOptions
  };

  return obj;
};

/*
 Generic  Canvas Layer for leaflet 0.7 and 1.0-rc,
 copyright Stanislav Sumbera,  2016 , sumbera.com , license MIT
 originally created and motivated by L.CanvasOverlay  available here: https://gist.github.com/Sumbera/11114288

 */

// -- L.DomUtil.setTransform from leaflet 1.0.0 to work on 0.0.7
//------------------------------------------------------------------------------
if (!L.DomUtil.setTransform) {
  L.DomUtil.setTransform = function (el, offset, scale) {
    var pos = offset || new L.Point(0, 0);

    el.style[L.DomUtil.TRANSFORM] =
      (L.Browser.ie3d
        ? "translate(" + pos.x + "px," + pos.y + "px)"
        : "translate3d(" + pos.x + "px," + pos.y + "px,0)") +
      (scale ? " scale(" + scale + ")" : "");
  };
}

// -- support for both  0.0.7 and 1.0.0 rc2 leaflet
L.CanvasLayer = (L.Layer ? L.Layer : L.Class).extend({
  // -- initialized is called on prototype
  initialize: function (options) {
    this._map = null;
    this._canvas = null;
    this._frame = null;
    this._delegate = null;
    L.setOptions(this, options);
  },

  delegate: function (del) {
    this._delegate = del;
    return this;
  },

  needRedraw: function () {
    if (!this._frame) {
      this._frame = L.Util.requestAnimFrame(this.drawLayer, this);
    }
    return this;
  },

  //-------------------------------------------------------------
  _onLayerDidResize: function (resizeEvent) {
    this._canvas.width = resizeEvent.newSize.x;
    this._canvas.height = resizeEvent.newSize.y;
  },
  //-------------------------------------------------------------
  _setCanvasPos: function () {
    var topLeft = this._map.containerPointToLayerPoint([0, 0]);
    L.DomUtil.setPosition(this._canvas, topLeft);
  },
  _onLayerDidMove: function (e) {
    var self = this;
    self.drawLayer(function () {
      self._setCanvasPos();
      self._frame = null;
      L.Util.cancelAnimFrame(self._frame);
    });
  },
  //-------------------------------------------------------------
  getEvents: function () {
    var events = {
      resize: this._onLayerDidResize,
      moveend: this._onLayerDidMove,
//      dragend: this._onLayerDidMove,
    };
    if (this._map.options.zoomAnimation && L.Browser.any3d) {
      events.zoomanim = this._animateZoom;
    }

    return events;
  },
  //-------------------------------------------------------------
  onAdd: function (map) {
    //console.log('canvas onAdd', this);
    this._map = map;
    this._canvas = L.DomUtil.create("canvas", "leaflet-layer");
    this._canvas.style.pointerEvents = 'none';
    this.tiles = {};

    var size = this._map.getSize();
    this._canvas.width = size.x;
    this._canvas.height = size.y;

    var animated = this._map.options.zoomAnimation && L.Browser.any3d;
    L.DomUtil.addClass(
      this._canvas,
      "leaflet-zoom-" + (animated ? "animated" : "hide")
    );

    this.options.pane.appendChild(this._canvas);
    map.on(this.getEvents(), this);
    this._setCanvasPos();

    var del = this._delegate || this;
    del.onLayerDidMount && del.onLayerDidMount(); // -- callback

    this.needRedraw();
  },

  //-------------------------------------------------------------
  onRemove: function (map) {
    var del = this._delegate || this;
    del.onLayerWillUnmount && del.onLayerWillUnmount(); // -- callback
    this.options.pane.removeChild(this._canvas);
    map.off(this.getEvents(), this);
    this._canvas = null;
  },

  //------------------------------------------------------------
  addTo: function (map) {
    map.addLayer(this);
    return this;
  },

  //------------------------------------------------------------------------------
  drawLayer: function (cb) {
    // -- todo make the viewInfo properties  flat objects.
    var size = this._map.getSize();
    var bounds = this._map.getBounds();
    var zoom = this._map.getZoom();

    var center = this._map.options.crs.project(this._map.getCenter());
    var corner = this._map.options.crs.project(
      this._map.containerPointToLatLng(this._map.getSize())
    );

    var del = this._delegate || this;
    del.onDrawLayer &&
      del.onDrawLayer({
        layer: this,
        canvas: this._canvas,
        bounds: bounds,
        size: size,
        zoom: zoom,
        center: center,
        corner: corner
      }, null, cb);
    if(!del.onDrawLayer) cb;
  },
  // -- L.DomUtil.setTransform from leaflet 1.0.0 to work on 0.0.7
  //------------------------------------------------------------------------------
  _setTransform: function (el, offset, scale) {
    var pos = offset || new L.Point(0, 0);

    el.style[L.DomUtil.TRANSFORM] =
      (L.Browser.ie3d
        ? "translate(" + pos.x + "px," + pos.y + "px)"
        : "translate3d(" + pos.x + "px," + pos.y + "px,0)") +
      (scale ? " scale(" + scale + ")" : "");
  },

  //------------------------------------------------------------------------------
  _animateZoom: function (e) {
    var scale = this._map.getZoomScale(e.zoom);
    // -- different calc of offset in leaflet 1.0.0 and 0.0.7 thanks for 1.0.0-rc2 calc @jduggan1
    var offset = L.Layer
      ? this._map._latLngToNewLayerPoint(
        this._map.getBounds().getNorthWest(),
        e.zoom,
        e.center
      )
      : this._map
        ._getCenterOffset(e.center)
        ._multiplyBy(-scale)
        .subtract(this._map._getMapPanePos());

    L.DomUtil.setTransform(this._canvas, offset, scale);
  }
});

L.canvasLayer = function (pane) {
  return new L.CanvasLayer(pane);
};
L.Control.Velocity = L.Control.extend({
  options: {
    position: "bottomleft",
    emptyString: "Unavailable",
    // Could be any combination of 'bearing' (angle toward which the flow goes) or 'meteo' (angle from which the flow comes)
    // and 'CW' (angle value increases clock-wise) or 'CCW' (angle value increases counter clock-wise)
    angleConvention: "bearingCCW",
    // Could be 'm/s' for meter per second, 'k/h' & 'km/h' for kilometer per hour or 'kt' & 'kn' for knots, 'mph' for miles per hour.
    speedUnit: "m/s",
    onAdd: null,
    onRemove: null
  },

  onAdd: function(map) {
    this._container = L.DomUtil.create("div", "leaflet-control-velocity");
    L.DomEvent.disableClickPropagation(this._container);
    map.on("mousemove", this._onMouseMove, this);
    this._container.innerHTML = this.options.emptyString;
    if (this.options.leafletVelocity.options.onAdd)
      this.options.leafletVelocity.options.onAdd();
    return this._container;
  },

  onRemove: function(map) {
    map.off("mousemove", this._onMouseMove, this);
    if (this.options.leafletVelocity.options.onRemove)
      this.options.leafletVelocity.options.onRemove();
  },

  vectorToSpeed: function(uMs, vMs, unit) {
    var velocityAbs = Math.sqrt(Math.pow(uMs, 2) + Math.pow(vMs, 2));
    switch (unit) {
    case 'k/h':
    case 'km/h':
      return this.meterSec2kilometerHour(velocityAbs);
    case 'kt':
    case 'kn':
      return this.meterSec2Knots(velocityAbs);
    case 'mph':
      return this.meterSec2milesPerHour(velocityAbs);
    default: // Default is m/s
    }
    return velocityAbs;
  },

  vectorToDegrees: function(uMs, vMs, angleConvention) {
    // Default angle convention is CW
    if (angleConvention.endsWith("CCW")) {
      // vMs comes out upside-down..
      vMs = vMs > 0 ? (vMs = -vMs) : Math.abs(vMs);
    }
    var velocityAbs = Math.sqrt(Math.pow(uMs, 2) + Math.pow(vMs, 2));

    var velocityDir = Math.atan2(uMs / velocityAbs, vMs / velocityAbs);
    var velocityDirToDegrees = (velocityDir * 180) / Math.PI + 180;

    if (angleConvention === "bearingCW" || angleConvention === "meteoCCW") {
      velocityDirToDegrees += 180;
      if (velocityDirToDegrees >= 360) velocityDirToDegrees -= 360;
    }

    return velocityDirToDegrees;
  },

  meterSec2Knots: function(meters) {
    return meters / 0.514;
  },

  meterSec2kilometerHour: function(meters) {
    return meters * 3.6;
  },
  meterSec2milesPerHour: function(meters) {
    return meters * 2.236936;
  },

  _onMouseMove: function(e) {
    var self = this;
    var pos = this.options.leafletVelocity._map.containerPointToLatLng(
      L.point(e.containerPoint.x, e.containerPoint.y)
    );
    var gridValue = this.options.leafletVelocity._windy.interpolatePoint(
      pos.lng,
      pos.lat
    );
    var htmlOut = "";

    if (
      gridValue &&
      !isNaN(gridValue[0]) &&
      !isNaN(gridValue[1]) &&
      gridValue[2]
    ) {
      htmlOut =
        "<strong>" +
        this.options.velocityType +
        " Direction: </strong>" +
        self
          .vectorToDegrees(
            gridValue[0],
            gridValue[1],
            this.options.angleConvention
          )
          .toFixed(2) +
        "°" +
        ", <strong>" +
        this.options.velocityType +
        " Speed: </strong>" +
        self
          .vectorToSpeed(gridValue[0], gridValue[1], this.options.speedUnit)
          .toFixed(2) +
        this.options.speedUnit;
    } else {
      htmlOut = this.options.emptyString;
    }

    self._container.innerHTML = htmlOut;
  }
});

L.Map.mergeOptions({
  positionControl: false
});

L.Map.addInitHook(function() {
  if (this.options.positionControl) {
    this.positionControl = new L.Control.MousePosition();
    this.addControl(this.positionControl);
  }
});

L.control.velocity = function(options) {
  return new L.Control.Velocity(options);
};
L.GradientLayer = (L.Layer ? L.Layer : L.Class).extend({
  options: {
    colorScale: null,
    data: null,
    reverseX: false,
    reverseY: false,
    dpx: 2, // 2x2 px
    getValue: null, // function(lat, lon) return value
  },

  _map: null,
  _canvasLayer: null,
  _context: null,
  _mouseControl: null,
  _init: null,
  _timer: 0, // delay draw
  _width: 0,
  _height: 0,

  initialize: function(options) {
    L.setOptions(this, options);
  },

  onAdd: function(map) {
    // determine where to add the layer
    this._paneName = this.options.paneName || "overlayPane";

    // fall back to overlayPane for leaflet < 1
    let pane = map._panes.overlayPane;
    if (map.getPane) {
      // attempt to get pane first to preserve parent (createPane voids this)
      pane = map.getPane(this._paneName);
      if (!pane) {
        pane = map.createPane(this._paneName);
      }
    }
    // create canvas, add to map pane
    this._canvasLayer = L.canvasLayer({ pane: pane }).delegate(this);
    this._canvasLayer.addTo(map);

    this._map = map;
  },

  onRemove: function(map) {
    this._destroy();
  },

  setData: function(data) {
    this.options.data = data;
    if (this._init) {
      this._init.setData(data);
      this._clearAndRestart();
    }
    this.fire("load");
  },

  setOpacity: function(opacity) {
    console.log("this._canvasLayer", this._canvasLayer);
    this._canvasLayer.setOpacity(opacity);
  },

  setOptions: function(options) {
    this.options = Object.assign(this.options, options);
    if (options.hasOwnProperty("data")) this.options.data = options.data;
    if (this._init) {
      this._init.setOptions(options);
      if (options.hasOwnProperty("data")) this._init.setData(options.data);
      this._clearAndRestart();
    }
    //this._initMouseHandler(true);

    this.fire("load");
  },

  /*------------------------------------ PRIVATE ------------------------------------------*/

  onDrawLayer: function(overlay, params, doneCb) {
    var self = this;

    if (!this._init) {
      this._initLayer(this);
      return;
    }

    if (!this.options.data) {
      return;
    }

    if (this._timer) clearTimeout(self._timer);
    this._timer = setTimeout(function() {
      self._draw(doneCb);
    }, 250); // draw data is delayed
  },

  _initLayer: function(self) {
    self._resize();

    // copy options
    const options = Object.assign(
      { canvas: self._canvasLayer._canvas, map: this._map },
      self.options
    );
    this._init = new GradientCanvas(options);

    // prepare context global var, start drawing
    this._context = this._canvasLayer._canvas.getContext("2d");
    this._canvasLayer._canvas.classList.add("gradient-overlay");
    this.onDrawLayer();

    this._map.on("dragstart", self._init.stop);
    this._map.on("dragend", self._clearAndRestart);
    this._map.on("zoomstart", self._init.stop);
    this._map.on("zoomend", self._clearAndRestart);
    this._map.on("resize", self._resize);

    //this._initMouseHandler(false);
  },

  _initMouseHandler: function(voidPrevious) {
    if (voidPrevious) {
      this._map.removeControl(this._mouseControl);
      this._mouseControl = false;
    }
    if (!this._mouseControl && this.options.displayValues) {
      var options = this.options.displayOptions || {};
      options["leafletGradient"] = this;
      this._mouseControl = L.control.velocity(options).addTo(this._map);
    }
  },

  _clearAndRestart: function() {
    if (this._context) this._context.clearRect(0, 0, this._width, this._height);
    if (this._init) this._draw();
  },

  _draw: function(cb) {
    var bounds = this._map.getBounds();
    var size = this._map.getSize();

    // bounds, width, height, extent
    this._init.start(
      [
        [0, 0],
        [size.x, size.y]
      ],
      size.x,
      size.y,
      [
        [bounds._southWest.lng, bounds._southWest.lat],
        [bounds._northEast.lng, bounds._northEast.lat]
      ],
      cb
    );
  },

  _resize: function(e) {
    var size = (e) ? e.newSize : this._map.getSize();
    this._width = size.x;
    this._height = size.y;
    if (this._init) this._init.stop();
    if (this._context) this._context.clearRect(0, 0, this._width, this._height);
  },

  _destroy: function() {
    if (this._timer) clearTimeout(this._timer);
    if (this._init) this._init.stop();
    if (this._context) this._context.clearRect(0, 0, this._width, this._height);
    if (this._mouseControl) this._map.removeControl(this._mouseControl);
    this._mouseControl = null;
    this._init = null;
    this._map.removeLayer(this._canvasLayer);
  }
});

L.gradientLayer = function(options) {
  return new L.GradientLayer(options);
};
L.VelocityLayer = (L.Layer ? L.Layer : L.Class).extend({
  options: {
    displayValues: true,
    displayOptions: {
      velocityType: "Velocity",
      position: "bottomleft",
      emptyString: "No velocity data"
    },
    maxVelocity: 10, // used to align color scale
    colorScale: null,
    data: null,
    reverseX: false,
    reverseY: false,
    waveStyle: false,
  },

  _map: null,
  _canvasLayer: null,
  _windy: null,
  _context: null,
  _timer: 0,
  _mouseControl: null,

  initialize: function(options) {
    L.setOptions(this, options);
  },

  onAdd: function(map) {
    // determine where to add the layer
    this._paneName = this.options.paneName || "overlayPane";

    // fall back to overlayPane for leaflet < 1
    let pane = map._panes.overlayPane;
    if (map.getPane) {
      // attempt to get pane first to preserve parent (createPane voids this)
      pane = map.getPane(this._paneName);
      if (!pane) {
        pane = map.createPane(this._paneName);
      }
    }
    // create canvas, add to map pane
    this._canvasLayer = L.canvasLayer({ pane: pane }).delegate(this);
    this._canvasLayer.addTo(map);

    this._map = map;
  },

  onRemove: function(map) {
    this._destroyWind();
  },

  setData: function(data) {
    this.options.data = data;
    if (this._windy) {
      this._windy.setData(data);
      this._clearAndRestart();
    }
    this.fire("load");
  },

  setOpacity: function(opacity) {
    console.log("this._canvasLayer", this._canvasLayer);
    this._canvasLayer.setOpacity(opacity);
  },

  setOptions: function(options) {
    this.options = Object.assign(this.options, options);
    if (options.hasOwnProperty("displayOptions")) {
      this.options.displayOptions = Object.assign(
        this.options.displayOptions,
        options.displayOptions
      );
      this._initMouseHandler(true);
    }
    if (options.hasOwnProperty("data")) this.options.data = options.data;
    if (this._windy) {
      this._windy.setOptions(options);
      if (options.hasOwnProperty("data")) this._windy.setData(options.data);
      this._clearAndRestart();
    }

    this.fire("load");
  },

  /*------------------------------------ PRIVATE ------------------------------------------*/

  onDrawLayer: function(overlay, params, doneCb) {
    var self = this;

    if (!this._windy) {
      this._initWindy(this);
      return;
    }

    if (!this.options.data) {
      return;
    }

    if (this._timer) L.Util.cancelAnimFrame(self._timer);
    this._timer = L.Util.requestAnimFrame(this._clearAndRestart, this);

    if(doneCb && typeof doneCb === 'function') doneCb();
  },

  _startWindy: function() {
    //var topLeft = this._map.containerPointToLayerPoint([0, 0]);
    //L.DomUtil.setPosition(this._canvasLayer._canvas, topLeft);

    var bounds = this._map.getBounds();
    var size = this._map.getSize();

    // bounds, width, height, extent
    this._windy.start(
      [
        [0, 0],
        [size.x, size.y]
      ],
      size.x,
      size.y,
      [
        [bounds._southWest.lng, bounds._southWest.lat],
        [bounds._northEast.lng, bounds._northEast.lat]
      ]
    );
  },

  getEvents: function(){
      return {
        //dragstart:this._clearWind,
        //dragend:this._clearAndRestart,
        movestart:this._startDrag,
        moveend:this._stopDrag,
        zoomstart:this._clearWind,
        //zoomend:this._clearAndRestart,
        resize:this._clearAndRestart
      }
  },

  _initWindy: function(self) {
    // windy object, copy options
    const options = Object.assign(
      { canvas: self._canvasLayer._canvas, map: this._map },
      self.options
    );
    this._windy = new Windy(options);

    // prepare context global var, start drawing
    this._context = this._canvasLayer._canvas.getContext("2d");
    this._canvasLayer._canvas.classList.add("velocity-overlay");
    this.onDrawLayer();

    this._map.on(this.getEvents(),this)

    this._initMouseHandler(false);
  },

  _initMouseHandler: function(voidPrevious) {
    if (voidPrevious) {
      this._map.removeControl(this._mouseControl);
      this._mouseControl = false;
    }
    if (!this._mouseControl && this.options.displayValues) {
      var options = this.options.displayOptions || {};
      options["leafletVelocity"] = this;
      this._mouseControl = L.control.velocity(options).addTo(this._map);
    }
  },

  _startDrag: function(e) {
    if (this._windy) this._windy.startDrag();
  },
  _stopDrag: function(e) {
    if (this._windy) {
      this._windy.stopDrag();
      this._clearWind();
    }
  },

  _clearAndRestart: function(e) {
    if (this._context) this._context.clearRect(0, 0, 3000, 3000);
    if (this._windy) this._startWindy();
  },

  _clearWind: function(e) {
    if (this._windy) this._windy.stop();
    if (this._context) this._context.clearRect(0, 0, 3000, 3000);
  },

  _destroyWind: function() {
    if (this._timer) clearTimeout(this._timer);
    if (this._windy) this._windy.stop();
    if (this._context) this._context.clearRect(0, 0, 3000, 3000);
    if (this._mouseControl) this._map.removeControl(this._mouseControl);
    this._mouseControl = null;
    this._windy = null;
    this._map.removeLayer(this._canvasLayer);
  }
});

L.velocityLayer = function(options) {
  return new L.VelocityLayer(options);
};
/*  Global class for simulating the movement of particle through a 1km wind grid

 credit: All the credit for this work goes to: https://github.com/cambecc for creating the repo:
 https://github.com/cambecc/earth. The majority of this code is directly take nfrom there, since its awesome.

 This class takes a canvas element and an array of data (1km GFS from http://www.emc.ncep.noaa.gov/index.php?branch=GFS)
 and then uses a mercator (forward/reverse) projection to correctly map wind vectors in "map space".

 The "start" method takes the bounds of the map at its current extent and starts the whole gridding,
 interpolation and animation process.
 */

var Windy = function(params) {
	var self = this;

	var MIN_VELOCITY_INTENSITY = params.minVelocity || 0; // velocity at which particle intensity is minimum (m/s)
	var MAX_VELOCITY_INTENSITY = params.maxVelocity || 10; // velocity at which particle intensity is maximum (m/s)
	var VELOCITY_SCALE =
		(params.velocityScale || 0.005) *
		(Math.pow(window.devicePixelRatio, 1 / 3) || 1); // scale for wind velocity (completely arbitrary--this value looks nice)
	var MAX_PARTICLE_AGE = params.particleAge || 90; // max number of frames a particle is drawn before regeneration
	var MIN_PARTICLE_AGE = params.particleMinAge || 10; // min number of frames a particle is drawn before regeneration
	var PARTICLE_LINE_WIDTH = params.lineWidth || 1; // line width of a drawn particle
	var PARTICLE_MULTIPLIER = params.particleMultiplier || 1 / 300; // particle count scalar (completely arbitrary--this values looks nice)
	var PARTICLE_REDUCTION = Math.pow(window.devicePixelRatio, 1 / 3) || 1.6; // multiply particle count for mobiles by this amount
	var FRAME_RATE = params.frameRate || 15;
	var FRAME_TIME = 1000 / FRAME_RATE; // desired frames per second
	var OPACITY = params.opacity || 0.97;
	var reverseX = params.reverseX || false;
	var reverseY = params.reverseY || false;
	var waveStyle = params.waveStyle || false; // particle color set by particle age for true, by intensity for false
	var dataFn = params.dataFn || null; // function for data convert

	var defaulColorScale = [
		"rgb( 36,104,180)",
		"rgb( 60,157,194)",
		"rgb(128,205,193)",
		"rgb(151,218,168)",
		"rgb(198,231,181)",
		"rgb(238,247,217)",
		"rgb(255,238,159)",
		"rgb(252,217,125)",
		"rgb(255,182,100)",
		"rgb(252,150, 75)",
		"rgb(250,112, 52)",
		"rgb(245, 64, 32)",
		"rgb(237, 45, 28)",
		"rgb(220, 24, 32)",
		"rgb(180,  0, 35)"
	];

	const colorScale = params.colorScale || defaulColorScale;

	var NULL_WIND_VECTOR = [NaN, NaN, null]; // singleton for no wind in the form: [u, v, magnitude]

	self.grid = null;
	var gridData = params.data;
	var date;

	var setData = function(data) {
		gridData = data;
	};

	var setOptions = function(options) {
		if (options.hasOwnProperty("minVelocity"))
			MIN_VELOCITY_INTENSITY = options.minVelocity;

		if (options.hasOwnProperty("maxVelocity"))
			MAX_VELOCITY_INTENSITY = options.maxVelocity;

		if (options.hasOwnProperty("velocityScale"))
			VELOCITY_SCALE =
			(options.velocityScale || 0.005) *
			(Math.pow(window.devicePixelRatio, 1 / 3) || 1);

		if (options.hasOwnProperty("particleAge"))
			MAX_PARTICLE_AGE = options.particleAge;

		if (options.hasOwnProperty("particleMinAge"))
			MIN_PARTICLE_AGE = options.particleMinAge;

		if (options.hasOwnProperty("lineWidth"))
			PARTICLE_LINE_WIDTH = options.lineWidth;

		if (options.hasOwnProperty("particleMultiplier"))
			PARTICLE_MULTIPLIER = options.particleMultiplier;

		if (options.hasOwnProperty("opacity")) OPACITY = +options.opacity;

		if (options.hasOwnProperty("frameRate")) FRAME_RATE = options.frameRate;
			FRAME_TIME = 1000 / FRAME_RATE;

		if (options.hasOwnProperty("waveStyle")) waveStyle = options.waveStyle;

		if (options.hasOwnProperty("dataFn")) dataFn = options.dataFn;
	};


	/**
	 * @returns {Boolean} true if the specified value is not null and not undefined.
	 */
	var isValue = function(x) {
		return x !== null && x !== undefined;// && !isNaN(x);
	};

	/**
	 * @returns {Number} the value x clamped to the range [low, high].
	 */
	var clamp = function(x, range) {
		return Math.max(range[0], Math.min(x, range[1]));
	};

	/**
	 * @returns {Boolean} true if agent is probably a mobile device. Don't really care if this is accurate.
	 */
	var isMobile = function() {
		return /android|blackberry|iemobile|ipad|iphone|ipod|opera mini|webos/i.test(
			navigator.userAgent
		);
	};

	/**
	 * Calculate distortion of the wind vector caused by the shape of the projection at point (x, y). The wind
	 * vector is modified in place and returned by this function.
	 */
	var distort = function(projection, λ, φ, x, y, scale, wind) {
		var u = wind[0] * scale;
		var v = wind[1] * scale;
		var d = distortion(projection, λ, φ, x, y);

		// Scale distortion vectors by u and v, then add.
		wind[0] = d[0] * u + d[2] * v;
		wind[1] = d[1] * u + d[3] * v;
		return wind;
	};

	var distortion = function(projection, λ, φ, x, y) {
		var τ = 2 * Math.PI;
		var H = Math.pow(10, -5.2); // 0.00000630957344480193
		//var H = 0.0000360;          // 0.0000360°φ ~= 4m  (from https://github.com/cambecc/earth/blob/master/public/libs/earth/1.0.0/micro.js#L13)
		//var H = 5; // ToDo:   Why does this work?
		var hλ = λ < 0 ? H : -H;
		var hφ = φ < 0 ? H : -H;

		//var pλ = project(φ, λ + hλ);
		//var pφ = project(φ + hφ, λ);
		const pλ = mapToCanvas(λ + hλ, φ);
		const pφ = mapToCanvas(λ, φ + hφ);

		// Meridian scale factor (see Snyder, equation 4-3), where R = 1. This handles issue where length of 1º λ
		// changes depending on φ. Without this, there is a pinching effect at the poles.
		var k = Math.cos((φ / 360) * τ);
		return [
			(pλ[0] - x) / hλ / k,
			(pλ[1] - y) / hλ / k,
			(pφ[0] - x) / hφ,
			(pφ[1] - y) / hφ
		];
	};

	/**
	* Project a point on the map
	* @param λ Longitude
	* @param φ Latitude
	* @return [x, y]
	*/
	var mapToCanvas = function(λ, φ) {
		const ymin = mercY(mapBounds.south);
		const ymax = mercY(mapBounds.north);
		const xFactor = canvasBound.width / ( mapBounds.east - mapBounds.west );
		const yFactor = canvasBound.height / ( ymax - ymin );

		let y = mercY(deg2rad(φ) );
		const x = (deg2rad(λ) - mapBounds.west) * xFactor;
		y = (ymax - y) * yFactor;
		return [x, y];
	};

	var mercY = function(φ) {
		return Math.log( Math.tan( φ / 2 + Math.PI / 4 ) );
	}

	function Header(data) {
		var self = this;
		self.uData;
		self.vData;
		self.scalar;

		var tmp;
		data.forEach(function(record) {
			switch (
				record.header.parameterCategory +
				"," +
				record.header.parameterNumber
			) {
			case "1,2":
			case "2,2":
				tmp = record;
				self.uData = record.data;
				break;
			case "1,3":
			case "2,3":
				self.vData = record.data;
				break;
			default:
				self.scalar = record.data;
			}
		});

		var supported = true;

		if (data.length < 2 ) supported = false;
		if (!supported) console.log("Windy Error: data must have at least two components (u,v)");

		var header = tmp.header;
		if (header.hasOwnProperty("gridDefinitionTemplate") && header.gridDefinitionTemplate != 0 ) supported = false;
		if (!supported) {
			console.log("Windy Error: Only data with Latitude_Longitude coordinates is supported");
		}
		supported = true;  // reset for futher checks

		this.λ0 = header.lo1;
		this.φ0 = header.la1; // the grid's origin (e.g., 0.0E, 90.0N)

		this.Δλ = header.dx;
		this.Δφ = header.dy; // distance between grid points (e.g., 2.5 deg lon, 2.5 deg lat)

		this.ni = header.nx;
		this.nj = header.ny; // number of grid points W-E and N-S (e.g., 144 x 73)

		if (header.hasOwnProperty("scanMode")) {
			var scanModeMask = header.scanMode.toString(2)
			scanModeMask = ('0'+scanModeMask).slice(-8);
			var scanModeMaskArray = scanModeMask.split('').map(Number).map(Boolean);

			if (scanModeMaskArray[0]) this.Δλ = -this.Δλ;
			if (scanModeMaskArray[1]) this.Δφ = -this.Δφ;
			if (scanModeMaskArray[2]) supported = false;
			if (scanModeMaskArray[3]) supported = false;
			if (scanModeMaskArray[4]) supported = false;
			if (scanModeMaskArray[5]) supported = false;
			if (scanModeMaskArray[6]) supported = false;
			if (scanModeMaskArray[7]) supported = false;
			if (!supported) console.log("Windy Error: Data with scanMode: "+header.scanMode+ " is not supported.");
		}
		this.date = new Date(header.refTime);
		this.date.setHours(this.date.getHours() + header.forecastTime);
	}

	Header.prototype.data = function(i, uData, vData) {
		return [this.uData[i], this.vData[i]];
	}


	function Grid(header, canvasBound, mapBounds, callback) {
		this.header = header;
		this.bounds = canvasBound;
		this.grid = null;

		this.λ0 = header.λ0;
		this.φ0 = header.φ0; // the grid's origin (e.g., 0.0E, 90.0N)

		this.Δλ = header.Δλ;
		this.Δφ = header.Δφ; // distance between grid points (e.g., 2.5 deg lon, 2.5 deg lat)

		this.ni = header.ni;
		this.nj = header.nj; // number of grid points W-E and N-S (e.g., 144 x 73)

		this.buildGrid(header)

		callback(this);
	}

	Grid.prototype.buildGrid = function(header) {
		var ni = header.ni;
		var nj = header.nj;

		this.grid = [];
		var grid = this.grid;
		var p = 0;
		var isContinuous = Math.floor(ni * this.Δλ) >= 360;

		const uData = header.uData;
		const vData = header.vData;

		for (var j = 0; j < nj; j++) {
			var row = [];
			for (var i = 0; i < ni; i++, p++) {
				if (reverseX) {
					row.unshift(header.data(p, uData, vData));
				} else {
					row.push(header.data(p, uData, vData));
				}
			}
			if (isContinuous) {
				// For wrapped grids, duplicate first column as last column to simplify interpolation logic
				if (reverseX) {
					row.unshift(row[row.length-1]);
				} else {
					row.push(row[0]);
				}
			}
			if (reverseY) {
				grid.unshift(row);
			} else {
				grid.push(row);
			}
		}
	}

	Grid.prototype.release = function() {
		this.grid = null;
		this.header = null;
	}

	Grid.prototype.get = function(x, y) {
		var row = this.grid[Math.round(x)];
		return (row && row[Math.round(y)]) || NULL_WIND_VECTOR;
	}

	Grid.prototype.interpolate = function(λ, φ) {
		if (!this.grid) return null;

		var i = this.floorMod(λ - this.λ0, 360) / this.Δλ; // calculate longitude index in wrapped range [0, 360)
		var j = (this.φ0 - φ) / this.Δφ; // calculate latitude index in direction +90 to -90

		var fi = Math.floor(i),
			ci = fi + 1;
		var fj = Math.floor(j),
			cj = fj + 1;

		var grid = this.grid;
		var row;
		if ((row = grid[fj])) {
			var g00 = row[fi];
			var g10 = row[ci];
			if (isValue(g00) && isValue(g10) && (row = grid[cj])) {
				var g01 = row[fi];
				var g11 = row[ci];
				if (isValue(g01) && isValue(g11)) {
					// All four points found, so interpolate the value.
					return this.bilinearInterpolateVector(i - fi, j - fj, g00, g10, g01, g11);
				}
			}
		}
		return null;
	};

	Grid.prototype.bilinearInterpolateVector = function(x, y, g00, g10, g01, g11) {
		var rx = 1 - x;
		var ry = 1 - y;
		var a = rx * ry,
		b = x * ry,
		c = rx * y,
		d = x * y;
		var u = g00[0] * a + g10[0] * b + g01[0] * c + g11[0] * d;
		var v = g00[1] * a + g10[1] * b + g01[1] * c + g11[1] * d;
		return [u, v, Math.sqrt(u * u + v * v)];
	}

	/**
	 * @returns {Number} returns remainder of floored division, i.e., floor(a / n). Useful for consistent modulo
	 *          of negative numbers. See http://en.wikipedia.org/wiki/Modulo_operation.
	 */
	Grid.prototype.floorMod = function(a, n) {
		return a - n * Math.floor(a / n);
	};

	Grid.prototype.randomize = function(p) {
		// UNDONE: this method is terrible
		var x, y;
		var bounds = this.bounds;
		var v;
		var safetyNet = 2;
		do {
			x = Math.round(Math.floor(Math.random() * bounds.width) + bounds.x);
			y = Math.round(Math.floor(Math.random() * bounds.height) + bounds.y);
			v = this.interpolate(x, y);
		} while ((v === null || v[2] === null) && safetyNet-- > 0);
		p.x = x;
		p.y = y;
		return p;
	};

	function Particule(age) {
		this.x;
		this.y;
		this.age = age;
		//this.maxAge;

		this.xt;
		this.yt;
		this.intensity;
	}
	Particule.prototype.maxAge = MAX_PARTICLE_AGE;
	Particule.prototype.add = function(v) {
		var p = this;
		// Path from (x,y) to (xt,yt) is visible, so add this particle to the appropriate draw bucket.
		p.xt = p.x + v[0];
		p.yt = p.y + v[1];
		return p;
	};



	var buildBounds = function(bounds, width, height) {
		var upperLeft = bounds[0];
		var lowerRight = bounds[1];
		var x = Math.round(upperLeft[0]); //Math.max(Math.floor(upperLeft[0], 0), 0);
		var y = Math.max(Math.floor(upperLeft[1], 0), 0);
		var xMax = Math.min(Math.ceil(lowerRight[0], width), width - 1);
		var yMax = Math.min(Math.ceil(lowerRight[1], height), height - 1);
		return {
			x: x,
			y: y,
			xMax: width,
			yMax: yMax,
			width: width,
			height: height
		};
	};

	var deg2rad = function(deg) {
		return (deg / 180) * Math.PI;
	};

	var invert = function(x, y, windy) {
		var latlon = params.map.containerPointToLatLng(L.point(x, y));
		return [latlon.lng, latlon.lat];
	};

	var project = function(lat, lon, windy) {
		var xy = params.map.latLngToContainerPoint(L.latLng(lat, lon));
		return [xy.x, xy.y];
	};

	var invertN = invert;
	var projectN = project;
	var dxy = L.DomUtil.getPosition(params.map._mapPane);;
	var invertD = function(x, y, windy) {
		var p = L.point(x, y);
		var latlon = params.map.layerPointToLatLng(p.subtract(dxy));
		return [latlon.lng, latlon.lat];
	};
	var projectD = function(lat, lon, windy) {
		var xy = params.map.latLngToLayerPoint(L.latLng(lat, lon)).add(dxy);
		return [xy.x, xy.y];
	};

	var context2D; // cache params.canvas.getContext("2d");
	var mapBounds;
	var canvasBound;
	var colorStyles; // for runtime
	var buckets; // for runtime
	var particles; // for runtime

	var initAnimate = function() {
		var particleCount = Math.round(
			canvasBound.width * canvasBound.height * PARTICLE_MULTIPLIER
		);
		if (isMobile()) {
			particleCount *= PARTICLE_REDUCTION;
		}

		var grid = self.grid;
		particles = [];
		for (var i = 0; i < particleCount; i++) {
			particles.push(
				grid.randomize(new Particule(Math.floor((Math.random() * (MAX_PARTICLE_AGE-MIN_PARTICLE_AGE)) + MIN_PARTICLE_AGE) + 0))
			);
		}

		frame();
	};

	function evolve() {
		var projection = {}; // map.crs used instead
		var mapArea = (mapBounds.south - mapBounds.north) * (mapBounds.west - mapBounds.east);
		var velocityScale = VELOCITY_SCALE * Math.pow(mapArea, 0.4);

		buckets.forEach(function(bucket) {
			bucket.length = 0;
		});

		particles.forEach(function(particle) {
			if (particle.age > MAX_PARTICLE_AGE) {
				self.grid.randomize(particle).age = 0;
			}

			var x = particle.x;
			var y = particle.y;
			var coord = invert(x, y);
			var λ = coord[0];
			var φ = coord[1];
			var v = self.grid.interpolate(λ, φ); // vector at current position
			if (v) {
				v = distort(projection, λ, φ, x, y, velocityScale, v);
				var m = v[2];
				if (m === null || m == 0) {
					particle.age = MAX_PARTICLE_AGE; // particle has escaped the grid, never to return...
				} else {
					particle.add(v);
					if(waveStyle) m = particle.age;
					buckets[colorStyles.indexFor(m)].push(particle);
				}
			}
			particle.age += 1;
		});
	}

	var draw = function (g) {
		// Fade existing particle trails.
		var prev = "lighter";
		g.globalCompositeOperation = "destination-in";
		g.fillRect(canvasBound.x, canvasBound.y, canvasBound.width, canvasBound.height);
		g.globalCompositeOperation = prev;
		g.globalAlpha = OPACITY === 0 ? 0 : OPACITY * 0.9;

		// Draw new particle trails.
		buckets.forEach(function(bucket, i) {
			if (bucket.length > 0) {
				g.beginPath();
				g.strokeStyle = colorStyles[i];
				bucket.forEach(function(particle) {
					g.moveTo(particle.x, particle.y);
					g.lineTo(particle.xt, particle.yt);
					particle.x = particle.xt;
					particle.y = particle.yt;
				});
				g.stroke();
			}
		});
	}

	var animationLoop;
	var then = Date.now();
	var frame = function () {
		animationLoop = requestAnimationFrame(frame);
		var now = Date.now();
		var delta = now - then;
		if (delta > FRAME_TIME) {
			then = now - (delta % FRAME_TIME);
			evolve();
			draw(context2D);
		}
	};

	var start = function(bounds, width, height, extent) {
		mapBounds = {
			south: deg2rad(extent[0][1]),
			north: deg2rad(extent[1][1]),
			east: deg2rad(extent[1][0]),
			west: deg2rad(extent[0][0]),
			width: width,
			height: height
		};

		stop();

		function windIntensityColorScale(min, max) {
			colorScale.indexFor = function(m) {
				// map velocity speed to a style
				return Math.max(
					0,
					Math.min(
						colorScale.length - 1,
						Math.round(((m - min) / (max - min)) * (colorScale.length - 1))
					)
				);
			};

			return colorScale;
		}

		colorStyles = windIntensityColorScale(
			MIN_VELOCITY_INTENSITY,
			MAX_VELOCITY_INTENSITY
		);
		if (waveStyle) {
			colorStyles = windIntensityColorScale(
				MIN_PARTICLE_AGE,
				MAX_PARTICLE_AGE
			);
		}

		buckets = colorStyles.map(function() {
			return [];
		});

		var fadeFillStyle = `rgba(0, 0, 0, ${OPACITY})`;
		context2D = params.canvas.getContext("2d");
		context2D.lineWidth = PARTICLE_LINE_WIDTH;
		context2D.fillStyle = fadeFillStyle;
		context2D.globalAlpha = 0.6;

		canvasBound = buildBounds(bounds, width, height);

		// build grid
		var header = new Header(gridData);
		if(dataFn && typeof dataFn === 'function') header.data = dataFn;

		new Grid(header, canvasBound, mapBounds, function(grid) {
			self.grid = grid;
			initAnimate();
		})
	};

	var stop = function() {
		if (self.grid) self.grid.release();
		if (animationLoop) L.Util.cancelAnimFrame(animationLoop);
	};

	var startDrag = function() {
		dxy = L.DomUtil.getPosition(params.map._mapPane);
		invert = invertD;
		project = projectD;
	};
	var stopDrag = function() {
		invert = invertN;
		project = projectN;
	};

	self.params = params;
	self.start = start;
	self.stop = stop;
	self.draw = draw;
	self.setData = setData;
	self.setOptions = setOptions;

	self.startDrag = startDrag;
	self.stopDrag = stopDrag;

	self.interpolatePoint = function(λ, φ) { // for display
		if(!self.grid) return null;
		return self.grid.interpolate(λ, φ)
	};

	return self;
};





