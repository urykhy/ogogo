#!/usr/bin/env python3

import yaml

def withResponse(resp):
    for x in resp:
        if "schema" in resp[x]:
            return True
    return False

filename="urykhy1-queue-1.0.0-swagger.yaml"
with open(filename, 'r') as stream:
    doc = yaml.safe_load(stream)

print ("/*")
print (doc["info"]["title"])
print ("")
print (doc["info"]["description"])
print ("*/")
print ("package main")
print ("import \"github.com/gorilla/mux\"")
print ("import log \"github.com/sirupsen/logrus\"")
print ("")
print ("// CreateRouter creates swagger api router")
print ("func CreateRouter(log *log.Logger) *mux.Router {")
print ("r := mux.NewRouter()")
for fname in doc["paths"]:
    for method in doc["paths"][fname]:
        d = doc["paths"][fname][method]
        print ("r.Path(\"{}\").HandlerFunc(func (w http.ResponseWriter, r *http.Request)".format(doc["basePath"] + fname),"{")
        print ("// {}".format(d["summary"]))
        params = []
        if "parameters" in d:
            print ("q := r.URL.Query()")
            for param in d["parameters"]:
                #print (param)
                name = ''.join(x for x in param["name"].title() if not x == "_")
                name = name.replace("Id","ID")
                print ("var {} *{}".format(name, "int64" if param["type"] == "integer" else "string"))
                print ("{")
                print ("_, ok := q[\"{}\"]".format(param["name"]))
                print ("if ok {")
                if param["type"] == "integer":
                    print ("{}Tmp, err := strconv.ParseInt(q.Get(\"{}\"), 10, 64)".format(name, param["name"]))
                    print ("if err != nil {")
                    print ("log.WithField(\"method\", \"{}\").Warn(\"bad param {}\")".format(fname, param["name"]))
                    print ("w.WriteHeader(http.StatusBadRequest)")
                    print ("return")
                    print ("}")
                    print ("{0} = &{0}Tmp".format(name))
                else:
                    print ("{}Tmp := q.Get(\"{}\")".format(name, param["name"]))
                    print ("{0} = &{0}Tmp".format(name))
                if "required" in param and param["required"] == True:
                    print ("} else {")
                    print ("log.WithField(\"method\", \"{}\").Warn(\"no required param {}\")".format(fname, param["name"]))
                    print ("w.WriteHeader(http.StatusBadRequest)")
                    print ("return")
                print ("}")
                print ("}")
                params.append("&{}".format(name))

        if "responses" in d and withResponse(d["responses"]):
            wr = True
            print ("code, resp, err := {}({})".format(d["operationId"], ",".join(params)))
        else:
            wr = False
            print ("code, err := {}({})".format(d["operationId"], ",".join(params)))
        print ("if err != nil {")
        print ("w.WriteHeader(http.StatusInternalServerError)")
        print ("return")
        print ("}")
        if wr:
            print ("jresp, err := json.Marshal(resp)")
            print ("if err != nil {")
            print ("log.WithField(\"method\", \"{}\").Error(\"fail to format result\")".format(fname))
            print ("w.WriteHeader(http.StatusInternalServerError)")
            print ("return")
            print ("}")
            print ("w.Write(jresp)")
        print ("w.WriteHeader(code)")
        print ("})")
    print ("")
print ("return r")
print ("}")
# func handleDump(w http.ResponseWriter, r *http.Request)
