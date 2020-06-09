'use strict';

function byte2Size(bytes, decimals = 2) {
	if (bytes === 0) return '0 Bytes';

	const k = 1024;
	const dm = decimals < 0 ? 0 : decimals;
	const sizes = ['Bytes', 'KB', 'MB', 'GB', 'TB', 'PB', 'EB', 'ZB', 'YB'];

	const i = Math.floor(Math.log(bytes) / Math.log(k));

	return parseFloat((bytes / Math.pow(k, i)).toFixed(dm)) + ' ' + sizes[i];
}

function pand(num, decimals = 2) {
	var out = '' + num
	const dm = (decimals-1 < 0) ? 0 : decimals-1;
	for (var i = Math.pow(10, dm); i > 1; i /= 10) {
		if (num < i) {
			out = '0' + out
		}
	}
	return out
}

function utc2localStr(utc) {
	var t = new Date(utc)
	var d = pand(t.getFullYear(), 4) + '/' + pand(t.getMonth() + 1) + '/' + pand(t.getDate())
	var tt = pand(t.getHours()) + ':' + pand(t.getMinutes()) + ':' + pand(t.getSeconds())
	return d + ' ' + tt
}

function mksvg(color, fillColor, fillOpacity, sz) {
	fillColor = fillColor || color;
	fillOpacity = fillOpacity || 0.2;
	sz = sz || 24;
	return '<svg height="'+sz+'" width="'+sz+'" xmlns="http://www.w3.org/2000/svg" xmlns:xlink="http://www.w3.org/1999/xlink">'
	+ '<rect width="'+sz+'" height="'+sz+'" style="stroke:'+color+';fill:'+fillColor+';fill-opacity:'+fillOpacity+';stroke-width:3" />'
	+ '</svg>';
}

var toolbarOptions = [
	[{ 'font': [] }],
	[{ 'size': ['small', false, 'large', 'huge'] }],  // custom dropdown
	['bold', 'italic', 'underline', 'strike'],        // toggled buttons
	[{ 'script': 'sub'}, { 'script': 'super' }],      // superscript/subscript
	[{ 'color': [] }, { 'background': [] }],          // dropdown with defaults from theme

	[{ 'align': [] }],
	[{ 'list': 'ordered'}, { 'list': 'bullet' }],
	[{ 'indent': '-1'}, { 'indent': '+1' }],          // outdent/indent
	['blockquote', 'code-block'],

	['link', 'image'],

	[{ 'header': [1, 2, 3, 4, 5, 6, false] }],

	['clean']                                         // remove formatting button
];

// sub UI
function setupSubUI(data, tmplText, extName, optClass, optType, toggleFn) {
	$('.'+ optClass).remove()
	var tmplFn = doT.template(tmplText)
	if(data) $('input[input-ext="'+ extName +'"]').parent().append(tmplFn(data))
	var fn = function (e) {
		var ele = $(this);
		var outE = ele.parent().parent().find('input[input-ext="'+ extName +'"]');
		toggleFn(e, ele, outE)
	}
	$('.'+ optClass +' '+ optType).off('change').on('change', fn).each(fn);
}

function updateNav(perm) {
console.log("[nav]", perm)
	if(perm < 0) {
		$('nav').removeClass('auth').addClass('unauth')
		return
	}
	$('nav').addClass('auth').removeClass('unauth')
}

$("#nav").click(function(){
	var el = $(this);
	if(el.text() == '☰'){
		el.text('🞬');
		$("a.nav.btn").addClass('show')
	} else {
		el.text('☰');
		$("a.nav.btn").removeClass('show')
	}
});

var __cache = {}
function infoUpdate(){
	$.ajax({
		url: "/api/auth",
		method: "GET",
		cache: false,
		success: function(d, textStatus, jqXHR){
			console.log("[info]ok", d, textStatus, jqXHR)

			__cache = d
			if(d.user) {
				updateNav(1)
			} else {
				page.redirect('/login')
			}

		},
		//error: function(jqXHR, textStatus, errorThrown){
		//	console.log("[info]err", textStatus, errorThrown)
		//},
		error: alertOrLogin,
	})
}
infoUpdate()

function lookupUpdate(el, id, list) {
	var ids = el.find('[data-lookup="'+ id +'"]')
	if (!ids.length) return

	var map = {}
	for (var i=0; i<list.length; i++) {
		var d = list[i]
		map[d[id]] = d.name
	}
	ids.each(function(idx) {
		var e = $(this)
		var v = e.text().split(',')
		var out = []
		for (var i=0; i<v.length; i++) {
			var d = map[v[i]] || '[無資料]'
			out.push(d)
		}
		e.text(out.join(','))
	})
}

function lookupUpdateAll(el, data) {
	if(data.user) lookupUpdate(el, 'uid', data.user)
}


// UI
var user = {
	login: function (ctx, next) {
		updateNav(-1)
		console.log("[login]", ctx)
		$('.page[data-url="login"]').show()
		$('[do="login"]').off('click', user.loginAjax).on('click', user.loginAjax)
	},
	loginAjax: function(){
		var accE = $('div[data-url="login"] input[name="acc"]')
		var pwdE = $('div[data-url="login"] input[name="pwd"]')
		var data = {
			acc: accE.val(),
			pwd: pwdE.val(),
		}
		pwdE.val('')
		console.log("[login]post", data.acc)
		$.ajax({
			url: "/api/login",
			method: "POST",
			cache: false,
			data: data,
			success: function(data, textStatus, jqXHR){
				console.log("[auth]ok", data, textStatus, jqXHR)
				accE.val('')
				infoUpdate() // update lookup table
				updateNav(1)
				page('/')
			},
			error: function(jqXHR, textStatus, errorThrown){
				console.log("[auth]err", textStatus, errorThrown)
				if (errorThrown.match('Forbidden')) {
					// TODO: no alert
					alert('帳號或密碼錯誤!')
				} else {
					alert('不明錯誤! ' + errorThrown)
				}
			},
		})
	},
	logout: function (ctx, next) {
		console.log("[logout]", ctx)
		$.ajax({
			url: "/api/logout",
			method: "POST",
			cache: false,
			complete: function(jqXHR, textStatus){
				updateNav(-1)
				next()
			},
		})
	},
	edit: function (ctx, next) {
		//console.log("[profile]", ctx)
		$.ajax({
			url: "/api/user",
			method: "GET",
			cache: false,
			success: function(data, textStatus, jqXHR){
				var info = JSON.parse(data)
				console.log("[profile]ok", info, textStatus, jqXHR)
				$('.page[data-url="user"] input[name="name"]').val(info.name)
				$('[do="userSave"]').off('click', user.editAjax).on('click', user.editAjax)
			},
			error: alertOrLogin,
		})
	},
	editAjax: function(e){
		var nameE = $('.page[data-url="user"] input[name="name"]')
		var pwdE = $('.page[data-url="user"] input[name="pwd"]')
		var pwd1E = $('.page[data-url="user"] input[name="pwd1"]')
		var pwd2E = $('.page[data-url="user"] input[name="pwd2"]')
		var data = {
			name: nameE.val(),
			pwd: pwdE.val(),
		}

		// chpwd
		if (pwd1E.val() != '') {
			var pwd1 = pwd1E.val()
			var pwd2 = pwd2E.val()
			pwd1E.val('')
			pwd2E.val('')
			if (pwd1 != pwd2) {
				// TODO: no alert
				alert('2次輸入的密碼不同!!')
				return
			}
			data.pwd2 = pwd1
		}

		pwdE.val('')
		$.ajax({
			url: "/api/user",
			method: "POST",
			cache: false,
			data: data,
			success: function(data, textStatus, jqXHR){
				var ret = JSON.parse(data)
				console.log("[user]ok", ret, textStatus, jqXHR)
				if (!ret.ok) {
					// TODO: no alert
					alert('錯誤:' + ret.msg)
					return
				}
				page('/')
			},
			error: alertOrLogin,
		})
	},
};
page('/', go('/layer/'))
page('/login', cknlog, showPage, user.login)
page('/logout', user.logout, go('/login'))
page('/user', showPage, user.edit)

var site = {
	edit: function(ctx, next){
		var el = $('.page[data-url="config"]')
		clrInput(el, site);
		el.find('[do="configSave"]').off('click', site.saveAjax).on('click', site.saveAjax)


		$.ajax({
			url: "/api/config/",
			method: "GET",
			cache: false,
			success: function(data, textStatus, jqXHR){
				var info = JSON.parse(data)
				console.log("[config]get", info, textStatus, jqXHR)

				//el.find('input[name="title"]').val(info.title)
				//el.find('input[name="logo"]').val(info.logo)
				setInput(el, info)
			},
			error: alertOrLogin,
		})
	},
	saveAjax: function(e){
		var el = $('.page[data-url="config"]')
		var titleE = el.find('input[name="title"]')
		var logoE = el.find('input[name="logo"]')
		var langE = el.find('input[name="lang"]')
		var manifestE = el.find('textarea[name="manifest"]')
		var headE = el.find('textarea[name="head"]')
		var loadfsE = el.find('input[name="loadfs"]')
		var data = {
			title: titleE.val(),
			logo: logoE.val(),
			lang: langE.val(),
			manifest: manifestE.val(),
			head: headE.val(),
			loadfs: parseInt(loadfsE.val()) || 0,

			cstats: (el.find('input[name="cstats"]').is(':checked')? '1' : ''),
			stats: (el.find('input[name="stats"]').is(':checked')? '1' : ''),
			link: (el.find('input[name="link"]').is(':checked')? '1' : ''),
			load: (el.find('input[name="load"]').is(':checked')? '1' : ''),
		}

		$.ajax({
			url: "/api/config/",
			method: "POST",
			cache: false,
			data: data,
			success: function(data, textStatus, jqXHR){
				var ret = JSON.parse(data)
				console.log("[config]set", ret, textStatus, jqXHR)
				if (!ret.ok) {
					// TODO: no alert
					alert('錯誤:' + ret.msg)
					return
				}
			},
			error: alertOrLogin,
		})
	},
}
page('/config', showPage, site.edit)
page('/status', showPage, function(ctx, next){
	var el = $('.page[data-url="status"]')
	clrInput(el, null);
	$.ajax({
		url: "/api/astats",
		method: "GET",
		cache: false,
		success: function(data, textStatus, jqXHR){
			console.log("[stats]get", data, textStatus, jqXHR)
			setInput(el, data)
		},
		error: alertOrLogin,
	})
})

function um2ajax(ele, isNew) {
	var ret = {}

	var accE = ele.find('input[name="acc"]')
	var pwd1E = ele.find('input[name="pwd1"]')
	var pwd2E = ele.find('input[name="pwd2"]')

	var nameE = ele.find('input[name="name"]')
	var noteE = ele.find('input[name="note"]')
	var suE = ele.find('input[name="su"]')
	var fzE = ele.find('input[name="fz"]')

	var data = {
		acc: accE.val(),
		name: nameE.val(),
		note: noteE.val(),

		su: (suE.is(':checked')? '1' : ''),
		fz: (fzE.is(':checked')? '1' : ''),
	}

	if (data.acc == '') {
		ret.err = '請輸入登入帳號!!'
		return ret
	}
	if (data.name == '') {
		ret.err = '請輸入使用者名稱!!'
		return ret
	}

	var pwd1 = pwd1E.val()
	var pwd2 = pwd2E.val()
	pwd1E.val('')
	pwd2E.val('')
	if (isNew) {
		// set pwd
		if (pwd1 == '') {
			ret.err = '請輸入使用者密碼!!'
			return ret
		}

		if (pwd1 != pwd2) {
			ret.err = '2次輸入的密碼不同!!'
			return ret
		}
		data.pwd = pwd1
	} else {
		// chpwd
		if (pwd1 != '') {
			if (pwd1 != pwd2) {
				// TODO: no alert
				alert('2次輸入的密碼不同!!')
				return
			}
			data.pwd = pwd1
		}
	}

	ret.data = data
	return ret
}
var um = mkUI($("#umlist").html(), 'usermanage', um2ajax)
page('/usermanage', showPage, um.list)
page('/usermanage/new', um.add)
page('/usermanage/:id', um.edit)



function layer2ajax(ele, isNew) {
	var ret = {}

	var nameE = ele.find('input[name="name"]')
	var noteE = ele.find('input[name="note"]')
	var attrE = ele.find('input[name="attr"]')
	var tokenE = ele.find('input[name="token"]')
	var fillcolorE = ele.find('input[name="fillcolor"]')
	var colorE = ele.find('input[name="color"]')
	var opacE = ele.find('input[name="opacity"]')
	var showE = ele.find('input[name="show"]')
	var hideE = ele.find('input[name="hide"]')
	var uvE = ele.find('input[name="uv"]')
	var data = {
		name: nameE.val(),
		note: noteE.val(),
		attr: attrE.val(),
		token: tokenE.val(),
		fillcolor: fillcolorE.val(),
		color: colorE.val(),
		opacity: opacE.val(),

		show: (showE.is(':checked')? '1' : ''),
		hide: (hideE.is(':checked')? '1' : ''),

		uv: (uvE.is(':checked')? '1' : ''),
	}

	if (data.name == '') {
		ret.err = '請輸入名稱!!'
		return ret
	}

	if (data.opacity == '') {
		data.opacity = 0.5
	}
	if ((data.opacity > 1.0) || (data.opacity < 0.0)) {
		ret.err = '透明度超過範圍(0.0~1.0)!!'
		return ret
	}

	ret.data = data
	return ret
}
var layer = mkUI($("#layerlist").html(), 'layer', layer2ajax)
layer.addCbFn = layer.editCbFn = function (el) {
	var fillcolorE = el.find('input[name="fillcolor"]')
	var colorE = el.find('input[name="color"]')
	var opacityE = el.find('input[name="opacity"]')
	var opacityVE = el.find('input[value-bind="opacity"]')

	fillcolorE.off('change').on('change', update)
	colorE.off('change').on('change', update)
	opacityE.off('change').on('change', update)
	opacityVE.off('change').on('change', setE)
	update()
	function setE() {
		opacityE.val(opacityVE.val())
	}
	function update() {
		opacityVE.val(opacityE.val())
		var html = mksvg(colorE.val(), fillcolorE.val(), opacityE.val(), 32)
		el.find('.legend').html(html)
	}
}
page('/layer', showPage, layer.list)
page('/layer/new', layer.add)
page('/layer/:id', layer.edit)

function map2ajax(ele, isNew) {
	var ret = {}

	var nameE = ele.find('input[name="name"]')
	var noteE = ele.find('input[name="note"]')
	var attrE = ele.find('input[name="attr"]')
	var urlE = ele.find('input[name="url"]')
	var domainE = ele.find('input[name="subdomains"]')
	var errTileE = ele.find('input[name="errorTileUrl"]')
	var maxZoomE = ele.find('input[name="maxZoom"]')
	var hideE = ele.find('input[name="hide"]')
	var data = {
		name: nameE.val(),
		note: noteE.val(),
		attr: attrE.val(),
		url: urlE.val(),
		subdomains: domainE.val(),
		errorTileUrl: errTileE.val(),
		maxZoom: maxZoomE.val(),

		hide: (hideE.is(':checked')? '1' : ''),
	}

	if (data.name == '') {
		ret.err = '請輸入名稱!!'
		return ret
	}

	if (data.maxZoom == '') {
		data.maxZoom = 18
	}
	if ((data.maxZoom > 18) || (data.maxZoom < 0)) {
		ret.err = '最大縮放層級超過範圍(0~18)!!'
		return ret
	}

	ret.data = data
	return ret
}
var map = mkUI($("#maplist").html(), 'map', map2ajax)
page('/map', showPage, map.list)
page('/map/new', map.add)
page('/map/:id', map.edit)

function link2ajax(ele, isNew) {
	var ret = {}

	var nameE = ele.find('input[name="name"]')
	var noteE = ele.find('input[name="note"]')
	var titleE = ele.find('input[name="title"]')
	var urlE = ele.find('input[name="url"]')
	var hideE = ele.find('input[name="hide"]')
	var data = {
		name: nameE.val(),
		note: noteE.val(),
		title: titleE.val(),
		url: urlE.val(),

		hide: (hideE.is(':checked')? '1' : ''),
	}

	if (data.name == '') {
		ret.err = '請輸入名稱!!'
		return ret
	}

	ret.data = data
	return ret
}
var link = mkUI($("#linklist").html(), 'link', link2ajax)
link.orderIndentP = function(ev){
	var el = $(this).parent().parent();
	var lv = parseInt(el.attr('data-indent'));
	var preE = el.prev();
	var preLv = parseInt(preE.attr('data-indent'));
	if (lv <= preLv) {
		el.attr('data-indent', lv+1)
		el.find('[data-val]').text(lv+1)
	}
}
link.orderIndentM = function(ev){
	var el = $(this).parent().parent();
	var lv = parseInt(el.attr('data-indent'));
	if(lv > 0) {
		el.attr('data-indent', lv-1)
		el.find('[data-val]').text(lv-1)
	}
}
link.orderCbFn = function(el){ // orderFix
	console.log('[link]orderFix', el, el.parent())
	var links = el.parent().find('[data-indent]')
	var firstE = null
	links.each(function(idx) {
		var e = $(this)
		if (!firstE) {
			firstE = e;
			return
		}
		var lv = parseInt(e.attr('data-indent'))
		var lv0 = parseInt(firstE.attr('data-indent'))
		if ((lv - lv0) > 1) {
			e.attr('data-indent', lv0 + 1)
			e.find('[data-val="indent"]').text(lv0 + 1)
		}
		firstE = e;
	});
}
link.orderS = function(ev){
	var el = $('.page[data-url="link"]')
	var listE = el.find('[data-order][data-indent]')
	var list = [];
	listE.each(function(idx) {
		var e = $(this)
		var v = e.attr('data-order')
		var lv = e.attr('data-indent')
		list.push(v + '/' + lv)
	});
	console.log('[link]order', listE, list)
	$.ajax({
		url: '/api/link/order',
		method: "POST",
		cache: false,
		data: {'order': list.join(',')},
		success: function(data, textStatus, jqXHR){
			el.removeClass('order')

			// TODO: not json error
			var ret = JSON.parse(data)
			console.log('[link]ok', ret, textStatus, jqXHR)
			if (!ret.ok) {
				// TODO: no alert
				alert('錯誤:' + ret.msg)
				return
			}
			page('/link')
		},
		error: alertOrLogin,
	})
	el.removeClass('order')
}
link.listCbFn = function(el, info){
	el.find('[do="linkOrderIndentP"]').off('click', link.orderIndentP).on('click', link.orderIndentP)
	el.find('[do="linkOrderIndentM"]').off('click', link.orderIndentM).on('click', link.orderIndentM)
}
page('/link', showPage, link.list)
page('/link/new', link.add)
page('/link/:id', link.edit)

function tab2ajax(ele, isNew) {
	var ret = {}

	var titleE = ele.find('input[name="title"]')
	var noteE = ele.find('input[name="note"]')
	var iconE = ele.find('input[name="icon"]')
	var ciconE = ele.find('input[name="cicon"]')
	var showE = ele.find('input[name="show"]')
	var delta = tab.quill.getContents()
	var data = {
		title: titleE.val(),
		note: noteE.val(),
		icon: iconE.val(),
		cicon: ciconE.val(),
		data: JSON.stringify(delta),

		show: (showE.is(':checked')? '1' : ''),
	}

	if (data.title == '') {
		ret.err = '請輸入名稱!!'
		return ret
	}

	ret.data = data
	return ret
}
var tab = mkUI($("#tablist").html(), 'tab', tab2ajax)
tab.addCbFn = tab.editCbFn = function(el, ctx, did, ret){
	if (!tab.quill) {
		var qe = el.find('.quill-editor')
		var quill = new Quill(qe[0], {
			modules: {
				toolbar: {
					container: toolbarOptions,
				},
				history: {
					delay: 2000,
					maxStack: 500,
					userOnly: true
				}
			},
			placeholder: '請輸入內容...',
			theme: 'snow'
		})
		tab.quill = quill
	}
	if(ret && ret.data) tab.quill.setContents(JSON.parse(ret.data));
}
page('/tab', showPage, tab.list)
page('/tab/new', tab.add)
page('/tab/:id', tab.edit)

function perm2ajax(ele, isNew) {
	var ret = {}

	var nameE = ele.find('input[name="name"]')
	var noteE = ele.find('input[name="note"]')
	var permE = ele.find('input[name="perm"]')
	var data = {
		name: nameE.val(),
		note: noteE.val(),
		perm: permE.val(),
	}

	if (data.name == '') {
		ret.err = '請輸入名稱!!'
		return ret
	}

	ret.data = data
	return ret
}
var attach = mkUI($("#attachlist").html(), 'attach', function(){})
attach.upload = function (ctx, next) {
	var listFileFn = doT.template($('#dropper-file').html())

	var ele = $('.page[data-url="attach/edit"]')
	ele.show()

	var msgE = ele.find('.dropper-msg')
	msgE.html(listFileFn(null))

	var files = null;
	var inputE = ele.find('input[type="file"]');
	inputE.off('change').on('change', function(e){
		//console.log('[dropper]', e.target, this, this.files)
		msgE.html(listFileFn(this.files))
	}).val('')

	var dropperE = ele.find('.dropper')
	dropperE.off('click').on('click', function(e){
		//console.log('[dropper]', e.target, this, inputE)
		inputE.click()
	}).off('dragleave').on('dragleave', function(e){
		e.preventDefault();
		dropperE.removeClass('over');
	}).off('dragover').on('dragover', function(e){
		e.preventDefault();
		dropperE.addClass('over');
	}).off('drop').on('drop', function(e){
		//console.log('dropHandler', e, e.originalEvent.dataTransfer);
		e.preventDefault();
		dropperE.removeClass('over');
		files = e.originalEvent.dataTransfer.files;
		msgE.html(listFileFn(files))
	})
	$('[do="attachUpload"]').off('click').on('click', upload);

	function upload(){
		//console.log('[upload]', files, ele.find('input[type="file"]')[0].files);
		var form = new FormData()
		var f = files || ele.find('input[type="file"]')[0].files
		for (var i=0; i<f.length; i++) {
			form.append('attach', f[i])
		}

		nprogress.start();
		$.ajax({
			xhr: function() {
				var xhr = new window.XMLHttpRequest();

				xhr.upload.addEventListener("progress", function(evt) {
					if (evt.lengthComputable) {
						var percentComplete = evt.loaded / evt.total;
						nprogress.set(percentComplete);
						console.log('[upload]', percentComplete);
						if (percentComplete == 1) {
							nprogress.done();
							console.log('[upload] end')
						}
					}
				}, false);

				return xhr;
			},
			url: '/api/attach/',
			type: "POST",
			processData: false,
			contentType: false,
			mimeType: 'multipart/form-data',
			data: form,
			success: function(result) {
				console.log('[ajax][upload]', result);
				// TODO: no alert
				alert('上傳完成')
				page('/attach')
			},
			error: function (jqXHR, textStatus, errorThrown){
				console.log('[ajax][upload]err', textStatus, errorThrown, jqXHR, jqXHR.responseText);
			}
		});
	}
}
page('/attach', showPage, attach.list)
page('/attach/new', attach.upload)

page.base('/admin')
page.exit('*', closeModal)
page('*', notfound)
page();

function loger(ctx, next) {
	console.log("[enter]", ctx)
	next()
}

function cklog(ctx, next) {
	$.ajax({
		url: "/api/user",
		cache: false,
		dataType: 'json',
		success: function(data, textStatus, jqXHR){
			updateNav(1)
			next()
		},
		error: alertOrLogin,
	})
}

function cknlog(ctx, next) {
	$.ajax({
		url: "/api/user",
		cache: false,
		dataType: 'json',
		success: function(data, textStatus, jqXHR){
			updateNav(1)
			page.redirect('/')
		},
		error: function(jqXHR, textStatus, errorThrown){
			next()
		},
	})
}

function showPage(ctx, next) {
	console.log("[showPage]", ctx)
	var p = ctx.path
	var lc = p[p.length - 1]
	p = (lc == '/') ? p.slice(1, -1) : p.substr(1);
	$('.page[data-url="'+ p +'"]').show()
	next()
}

function alertOrLogin(jqXHR, textStatus, errorThrown){
	console.log("err", textStatus, errorThrown)
	if (errorThrown.match('no permission')) {
		// TODO: no alert
		alert('no permission!')
		return
	}
	/*if (errorThrown.match('Internal server error')) {
		// TODO: no alert
		alert(errorThrown)
		return
	}*/
	if (errorThrown.match('Forbidden')) {
		page.redirect('/login')
		return
	}
	// TODO: no alert
	alert(errorThrown)
}

function go(path) {
	return function(ctx, next){
		page(path)
	}
}

function closeModal(ctx, next) {
	$('.page').hide()
	$('.page[data-url] div.rTable').html('')
	//console.log("[closeModal]", ctx)
	next()
}

function notfound(ctx, next) {
	console.log("[notfound]", ctx)
}

$( document ).ajaxStart(function() {
	nprogress.start();
});
$( document ).ajaxStop(function() {
	nprogress.done();
});
function clrInput(el, obj) {
	el.find('input[type="text"]').val('')
	el.find('input[type="password"]').val('')
	el.find('input[type="checkbox"]').prop('checked', false)
	el.find('input[type="range"]').val('')
	el.find('input[type="color"]').val('')
	if (obj && obj.quill) {
		obj.quill.setText('')
	}
}
function setInput(el, data) {
	for(var k in data) {
		var e = el.find('[name="' + k + '"]')
		var trans = e.attr('data-transform')
		var v = data[k]
		switch (trans) {
		case 'localtime':
			v = utc2localStr(v)
			break
		}
		if (e.attr('type') == 'checkbox') {
			e.prop('checked', v)
		} else {
			e.val(v)
		}
		el.find('[data-name="' + k + '"]').text(v)
	}
}
function mkUI(tmplText, op, input2dataFn) {
	var obj = {
		order: function(ev){
			var el = $('.page[data-url="'+ op +'"]')
			el.addClass('order')
		},
		orderC: function(ev){
			var el = $('.page[data-url="'+ op +'"]')
			el.removeClass('order')
			obj.list()
		},
		orderU: function(ev){
			var e = $(this).parent().parent();
			var next = e.prev();
			console.log('['+op+']order up', e, next)
			e.after(next);
			if(obj.orderCbFn) obj.orderCbFn(e);
		},
		orderD: function(ev){
			var e = $(this).parent().parent();
			var next = e.next();
			console.log('['+op+']order down', e, next)
			e.before(next);
			if(obj.orderCbFn) obj.orderCbFn(e);
		},
		orderS: function(ev){
			var el = $('.page[data-url="'+ op +'"]')
			var listE = el.find('[data-order]')
			var list = [];
			listE.each(function(idx) {
				var e = $(this)
				var v = e.attr('data-order')
				list.push(v)
			});
			console.log('['+op+']order', listE, list)
			$.ajax({
				url: '/api/'+ op +'/order',
				method: "POST",
				cache: false,
				data: {'order': list.join(',')},
				success: function(data, textStatus, jqXHR){
					el.removeClass('order')

					// TODO: not json error
					var ret = JSON.parse(data)
					console.log('['+ op +']ok', ret, textStatus, jqXHR)
					if (!ret.ok) {
						// TODO: no alert
						alert('錯誤:' + ret.msg)
						return
					}
					page('/' + op)
				},
				error: alertOrLogin,
			})
			el.removeClass('order')
		},
		listTmpl: doT.template(tmplText),
		list: function (ctx, next) {
			console.log('['+op+']list', ctx)
			$.ajax({
				url: '/api/'+ op +'/',
				method: "GET",
				cache: false,
				success: function(data, textStatus, jqXHR){
					var info = JSON.parse(data)
					console.log('['+op+']list', info, textStatus, jqXHR)
					var el = $('.page[data-url="'+ op +'"] div.rTable')
					el.html(obj.listTmpl(info))
					lookupUpdateAll(el, __cache)
					$('[do="'+ op +'Del"]').off('click', obj.delAjax).on('click', obj.delAjax)
					$('[do="'+ op +'Order"]').off('click', obj.order).on('click', obj.order)
					$('[do="'+ op +'OrderCancel"]').off('click', obj.orderC).on('click', obj.orderC)
					$('[do="'+ op +'OrderSave"]').off('click', obj.orderS).on('click', obj.orderS)
					$('[do="'+ op +'OrderUp"]').off('click', obj.orderU).on('click', obj.orderU)
					$('[do="'+ op +'OrderDown"]').off('click', obj.orderD).on('click', obj.orderD)
					if(obj.listCbFn) obj.listCbFn(el, info)
				},
				error: alertOrLogin,
			})
		},
		add: function (ctx, next) {
			$('.page').hide()
console.log('['+op+']add', ctx)
			var el = $('.page[data-url="'+ op +'/edit"]')
			el.removeClass('edit').removeClass('new').addClass('new')
			clrInput(el, obj) // clear
			$('[do="'+ op +'Save"]').off('click', obj.postAjax).on('click', {id:''}, obj.postAjax)
			if(obj.addCbFn) obj.addCbFn(el, ctx)
			el.show()
		},
		edit: function (ctx, next) {
console.log('['+op+']edit', ctx)
			var el = $('.page[data-url="'+ op +'/edit"]')
			var did = ctx.params.id
			$.ajax({
				url: '/api/'+ op +'/' + did,
				method: "GET",
				cache: false,
				success: function(data, textStatus, jqXHR){
					var ret = JSON.parse(data)
					console.log('['+ op +']get', ret, textStatus, jqXHR)

					el.removeClass('edit').removeClass('new').addClass('edit')
					clrInput(el, obj) // clear

					if (ret instanceof Array) {
						for (var i=0; i<ret.length; i++) {
							var d = ret[i]
							var id = d.aid || d.lyid || d.mid || d.anid || d.liid || d.uid
							if (did == id) {
								setInput(el, d)
								ret = d
								break
							}
						}
					} else {
						setInput(el, ret)
					}

					$('[do="'+ op +'Save"]').off('click', obj.postAjax).on('click', {id:did}, obj.postAjax)
					if(obj.editCbFn) obj.editCbFn(el, ctx, did, ret)

					$('.page').hide()
					el.attr('data-id', did).show()
				},
				error: alertOrLogin,
			})
		},
		delAjax: function (e) {
			// TODO: no alert / confirm
			var ans = confirm('確定要刪除?')
			if (!ans) return

			//var el = $('.page[data-url="'+ op +'/edit"]')
			var el = $(this)
			var id = el.attr('data-id')
			console.log('['+ op +']del', e, this, id, el)

			$.ajax({
				url: '/api/'+ op +'/' + id + '/del',
				method: "POST",
				cache: false,
				success: function(data, textStatus, jqXHR){
					var ret = JSON.parse(data)
					console.log('['+ op +']del', ret, textStatus, jqXHR)
					if (!ret.ok) {
						// TODO: no alert
						alert('錯誤:' + ret.msg)
						return
					}
					page('/' + op)
					infoUpdate() // update lookup table
				},
				error: alertOrLogin,
			})
		},
		postAjax: function (e, isNew) {
			console.log('[postAjax]', e)
			var el = $('.page[data-url="'+ op +'/edit"]')
			var did = e.data.id
			var ret = input2dataFn(el, (e.data.id == ''))
			if (ret.err) {
				// TODO: no alert
				alert(ret.err)
				return
			}
			$.ajax({
				url: '/api/'+ op +'/' + did,
				method: "POST",
				cache: false,
				data: ret.data,
				success: function(data, textStatus, jqXHR){
					// TODO: not json error
					var ret = JSON.parse(data)
					console.log('['+ op +']ok', ret, textStatus, jqXHR)
					if (!ret.ok) {
						// TODO: no alert
						alert('錯誤:' + ret.msg)
						return
					}
					page('/' + op)
					infoUpdate() // update lookup table
				},
				error: alertOrLogin,
			})
		},
	};
	return obj
}

