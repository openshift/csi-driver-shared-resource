'''
This module is used as a wrapper for getting csi driver feature file attribute.
'''

from pyshould import should

class ShareResource(object):
    def __init__(self, resource, csi=None):
        if not csi:
            csi = []
        self.resource = resource
        self.csi = csi

    def add_csi(self, resource):
        assert resource not in self.csi
        self.csi.append(resource)

    def get_resource(self):
        return self.csi[0]

class ResourceTypeModel(object):
    def __init__(self):
        self.resource = []
        self.shareResource = {}

    def add_resource(self, resource, shareResource):
        assert resource not in self.resource
        if shareResource not in self.shareResource:
            self.shareResource[shareResource] = ShareResource(shareResource)
        self.shareResource[shareResource].add_csi(resource)

    def get_resource_type(self, shareResource):
        return self.shareResource[shareResource].get_resource()