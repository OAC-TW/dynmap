L.Control.Watermark = L.Control.extend({
	options: {
		position: 'topleft',
		image: 'res/logo.png',
		text: '',
		html: null,
		className: ''
	},
	onAdd: function(map) {
		var tagType = 'img';
		if ((this.options.image === '') && (this.options.html !== null)) {
			tagType = 'span';
		}
		var tag = L.DomUtil.create(tagType);
		if(this.options.image) tag.src = this.options.image;
		if(this.options.text) tag.textContent = this.options.text;
		if(this.options.html) tag.innerHTML = this.options.html;
		if(this.options.className) L.DomUtil.addClass(tag, this.options.className);
		this._container = tag;
		L.DomEvent.disableClickPropagation(this._container);
		return tag;
	},

	setText: function(data) {
		this.options.text = data;
		this._container.textContent = data;
	},
	setHtml: function(data) {
		this.options.html = data;
		this._container.innerHTML = data;
	},

	onRemove: function(map) {
		// Nothing to do here
	}
});

L.control.watermark = function(opts) {
	return new L.Control.Watermark(opts);
}

