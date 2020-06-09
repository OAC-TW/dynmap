'use strict';

/* global cw, shp */
L.Shapefile = L.GeoJSON.extend({
  options: {
    importUrl: 'shp.js'
  },

  initialize: function(file, options) {
    L.Util.setOptions(this, options);
    if (typeof cw !== 'undefined') {
      /*eslint-disable no-new-func*/
      if (!options.isArrayBuffer) {
        //this.worker = cw(new Function('data', 'cb', 'importScripts("' + this.options.importUrl + '");if(data == null) return this;shp(data).then(cb);'));
      } else {
        this.worker = cw(new Function('data', 'importScripts("' + this.options.importUrl + '"); return shp.parseZip(data);'));
      }
      /*eslint-enable no-new-func*/
    }
    L.GeoJSON.prototype.initialize.call(this, {
      features: []
    }, options);
    if(options.dry) {
        if (this.worker) {
            var self = this;
            this.worker.data(null).then(function(d){console.log('d', d)},function(data) {
                self.fire('worker:loaded');
console.log("finished loaded worker");
            })
        }
        return;
    }
    this.addFileData(file);
  },

  worker: cw(new Function('data', 'cb', 'importScripts("leaflet/shp.js");if(data == null) return this;shp(data).then(cb);')), // hacking

  addFileData: function(file) {
    var self = this;
    this.fire('data:loading');
    if (typeof file !== 'string' && !('byteLength' in file)) {
      var data = this.addData(file);
      this.fire('data:loaded');
      return data;
    }
    if (!this.worker) {
      shp(file).then(function(data) {
        self.addData(data);
        self.fire('data:loaded');
      }).catch(function(err) {
        self.fire('data:error', err);
      })
      return this;
    }
    var promise;
    if (this.options.isArrayBufer) {
      promise = this.worker.data(file, [file]);
    } else {
      promise = this.worker.data(cw.makeUrl(file));
    }

    promise.then(function(data) {
      self.addData(data);
      self.fire('data:loaded');
      if(self.worker != self.prototype.worker) self.worker.close();
    }, function(err) {
      self.fire('data:error', err);
    })
    return this;
  }
});

L.shapefile = function(a, b, c) {
  return new L.Shapefile(a, b, c);
};
