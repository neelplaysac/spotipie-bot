from sp_bot import SESSION
from bson import ObjectId


class MongoOperations:

    def __init__(self, SESSION):
        self.db = SESSION['spotipie']
        self.cursor1 = self.db['codes']
        self.cursor2 = self.db['users']
        self.cursor3 = self.db['lastfm']

    # 'Codes' database functions

    def fetchCode(self, _id):
        query = {'_id': _id}
        return self.cursor1.find_one(query)

    def deleteCode(self, _id):
        query = {'_id': _id}
        self.cursor1.delete_one(query)

    # 'Users' database functions

    def fetchData(self, tg_id):
        query = {'tg_id': tg_id}
        return self.cursor2.find_one(query)

    def updateData(self, tg_id, value):
        query = {'tg_id': tg_id}
        newvalues = {"$set": {"username": value}}
        self.cursor2.update_one(query, newvalues)

    def updateStyle(self, tg_id, value):
        query = {'tg_id': tg_id}
        newvalues = {"$set": {"style": value}}
        self.cursor2.update_one(query, newvalues)

    def deleteData(self, tg_id):
        query = {'tg_id': tg_id}
        self.cursor2.delete_one(query)

    def countAll(self):
        return self.cursor2.find().count()

    def aggregateUsers(self):
        pipeline = [
            {"$unwind": "$style"},
            {"$group": {"_id": "$style", "count": {"$sum": 1}}},
        ]
        result = list(self.cursor2.aggregate(pipeline))
        return result

    def addUser(self, tg_id, token):
        User = {
            "username": "User",
            "token": token,
            "style": "blur",
            "tg_id": tg_id
        }
        user = self.cursor2.insert_one(User)
        return user

    # 'lastFM' database functions

# -----------------------------------------------------------------------

    def addLastFmUser(self, tg_id, lastfm_username):
        User = {
            "fm_username": lastfm_username,
            "name": "User",
            "tg_id": tg_id,
            "counter": "on"
        }
        user = self.cursor3.insert_one(User)
        return user

    def getLastFmUser(self, tg_id):
        query = {'tg_id': tg_id}
        return self.cursor3.find_one(query)

    def updateLastFmData(self, tg_id, value):
        query = {'tg_id': tg_id}
        newvalues = {"$set": {"name": value}}
        self.cursor3.update_one(query, newvalues)

    def removeLastFmUser(self, tg_id):
        query = {'tg_id': tg_id}
        self.cursor3.delete_one(query)

    def countAllLastFm(self):
        return self.cursor3.find().count()

    def aggregateLastFmUsers(self):
        pipeline = [
            {"$unwind": "$counter"},
            {"$group": {"_id": "$counter", "count": {"$sum": 1}}},
        ]
        result = list(self.cursor3.aggregate(pipeline))
        return result

    def toggleCounter(self, tg_id, value):
        query = {'tg_id': tg_id}
        newvalues = {"$set": {"counter": value}}
        self.cursor3.update_one(query, newvalues)


DATABASE = MongoOperations(SESSION)
