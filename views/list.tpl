<div class="container-fluid">
  <div class="row">
    <div class="col-md-2">
      <!--Sidebar content-->

      Search: <input ng-model="query" value="mmstat">
      Sort by:
      <select ng-model="orderProp">
        <option value="domain">Host</option>
        <option value="id">Newest</option>
      </select>
      <br>
      <a href="#" ng-click="cleanIP()">Clean </a>
    </div>
    <div class="col-md-10">
      <!--Body content-->

      <ul class="phones">
        <li ng-repeat="phone in phones | filter:query | orderBy:orderProp" class="thumbnail">
          <a href="#/phones/{{phone.id}}" class="thumb">{{phone.url}}</a>
          <p>{{phone.desc}}</p>
        </li>
      </ul>

    </div>
  </div>
</div>
