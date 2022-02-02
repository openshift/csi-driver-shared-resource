import os
import random  
import string
import yaml

class Util:
    def random_string(self, letter_count, digit_count):
        str1 = (''.join((random.choice(string.ascii_letters) for x in range(letter_count)))).lower()
        str1 += ''.join((random.choice(string.digits) for x in range(digit_count)))
        str2 = list(str1)
        random.shuffle(str2)
        return ''.join(str2)

    def edit_resource_yaml_file(self, path, new_data, resource):
        temp_path = ""
        with open(path,'r') as yamlfile:
            config_resource = "\"share-config\""
            new_yaml = yaml.safe_load(yamlfile)
            if resource == config_resource:
                temp_path = "./_output/smoke/configmap.yaml"
                new_yaml['data'].update(new_data)
            else:
                temp_path = "./_output/smoke/secret.yaml"
                new_yaml.update(new_data)

        with open(temp_path,'w') as yamlfile:
            yaml.safe_dump(new_yaml, yamlfile)
        return temp_path
