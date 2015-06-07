var app = angular.module("cdkey", [])

app.controller("packctl", function($scope, $http) {
    $scope.addPack = function() {
    	$scope.add.keylen = Number($scope.add.keylen)
    	$scope.add.packsize = Number($scope.add.packsize)

    	$http.post("/pack.add", $scope.add).success(function(data) {
    		$scope.add = {}
    		$scope.reload()
    	}).error($scope.err)
    }

    $scope.deletePack = function(name) {
        $http.post("/pack.remove", {pack:name}).success(function(data) {
            $scope.reload()
        }).error($scope.err)
    }

    $scope.enablePack = function(name) {
        $http.post("/pack.enable", {pack:name}).success(function(data) {
            $scope.reload()
        }).error($scope.err)
    }

    $scope.disablePack = function(name) {
        $http.post("/pack.disable", {pack:name, msg:"disabled"}).success(function(data) {
            $scope.reload()
        }).error($scope.err)
    }

    $scope.reload = function() {
    	$http.post("/pack.list").success(function(data) {
    		$scope.packs = data.packs
    	}).error($scope.err)
    }

    $scope.err = function(data) {
    	alert("code " + data.code + "\n" + data.msg)
    }

    $scope.reload()
})