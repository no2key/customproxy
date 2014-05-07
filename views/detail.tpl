<style type="text/css">
.arg-stats_click {
	display: block;
	width:800px;
	border:1px solid gray;
	overflow:scroll;
}
</style>
<div style="padding:1em;">

<h1>{{phone.domain}}</h1>
<a href="{{phone.url}}">Jump</a>


		
        <span ng-repeat="param in phone.args " style="display:block;">
        	<span class="arg-{{param.key}}">
          {{param.key}}:
          {{param.val}}
      </span>
        </span>
      
        <span ng-repeat="param in phone.params " style="display:block;" >
        	<span class="arg-{{param.key}}">
          {{param.key}}:
          {{param.val}}
      </span>
        </span>

<pre>{{phone.body}}</pre>

</div>